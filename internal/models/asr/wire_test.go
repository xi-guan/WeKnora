package asr

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	modelruntime "github.com/Tencent/WeKnora/internal/models/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allowLoopback(t *testing.T) {
	t.Helper()
	t.Setenv("SSRF_WHITELIST", "127.0.0.1")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
}

type form struct {
	path     string
	auth     string
	fields   map[string]string
	fileName string
	language string // the language request header
	// body is set instead of fields when the request was JSON: the
	// chat-served recognisers take the audio as a data URI.
	body map[string]any
}

// upstream records the form it receives and answers the documented json
// shape, adding a segment when verbose_json was asked for.
func upstream(t *testing.T) (string, *form, *atomic.Int32) {
	t.Helper()
	allowLoopback(t)
	got := &form{fields: map[string]string{}}
	calls := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		got.path, got.auth, got.language = r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("language")
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		require.NoError(t, err)
		if mediaType == "application/json" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&got.body))
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":" hello "}}]}`))
			return
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			require.NoError(t, err)
			data, _ := io.ReadAll(part)
			if part.FormName() == "file" {
				got.fileName = part.FileName()
				continue
			}
			got.fields[part.FormName()] = string(data)
		}
		if got.fields["response_format"] == "verbose_json" {
			_, _ = w.Write([]byte(`{"text":" hello ","segments":[{"start":0,"end":1.5,"text":" hello "}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"text":" hello "}`))
	}))
	t.Cleanup(server.Close)
	return server.URL, got, calls
}

// TestTranscriptionWireFormatPerVendor pins the form each ASR vendor is sent.
// response_format appears only where the vendor documents it for that model:
// the pre-catalog client sent verbose_json to every one of them, which
// gpt-4o-transcribe rejects ("the only supported format is json") and which
// vox-box's FunASR backend answers with an undecodable bare string.
func TestTranscriptionWireFormatPerVendor(t *testing.T) {
	cases := []struct {
		name, provider, model, base string
		wantPath                    string
		wantFields                  map[string]string
		wantBody                    map[string]any
		wantSegments                int
	}{
		{
			name: "openai whisper-1 serves segments", provider: "openai", model: "whisper-1", base: "/v1",
			wantPath:     "/v1/audio/transcriptions",
			wantFields:   map[string]string{"model": "whisper-1", "response_format": "verbose_json"},
			wantSegments: 1,
		},
		{
			name: "openai gpt-4o-transcribe accepts only json", provider: "openai", model: "gpt-4o-transcribe",
			base: "/v1", wantPath: "/v1/audio/transcriptions",
			wantFields: map[string]string{"model": "gpt-4o-transcribe"},
		},
		{
			name: "openai model not yet in the catalog", provider: "openai", model: "gpt-5-transcribe",
			base: "/v1", wantPath: "/v1/audio/transcriptions",
			wantFields: map[string]string{"model": "gpt-5-transcribe"},
		},
		{
			name: "gpustack stays on json for its FunASR backend", provider: "gpustack",
			model: "SenseVoiceSmall", base: "/v1", wantPath: "/v1/audio/transcriptions",
			wantFields: map[string]string{"model": "SenseVoiceSmall"},
		},
		{
			name: "generic stays on json", provider: "generic", model: "whisper-large-v3",
			base: "/v1", wantPath: "/v1/audio/transcriptions",
			wantFields: map[string]string{"model": "whisper-large-v3"},
		},
		{
			name: "openrouter", provider: "openrouter", model: "openai/whisper-large-v3",
			base: "/api/v1", wantPath: "/api/v1/audio/transcriptions",
			wantFields: map[string]string{"model": "openai/whisper-large-v3"},
		},
		{
			name: "requesty whisper-1 serves segments", provider: "requesty", model: "openai/whisper-1",
			base: "/v1", wantPath: "/v1/audio/transcriptions",
			wantFields:   map[string]string{"model": "openai/whisper-1", "response_format": "verbose_json"},
			wantSegments: 1,
		},
		{
			name: "litellm proxy", provider: "litellm", model: "whisper",
			base: "/v1", wantPath: "/v1/audio/transcriptions",
			wantFields: map[string]string{"model": "whisper"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, got, _ := upstream(t)
			a, err := NewASR(&Config{
				Source: types.ModelSourceRemote, Provider: tc.provider, BaseURL: url + tc.base,
				ModelName: tc.model, APIKey: "k",
			})
			require.NoError(t, err)
			out, err := a.Transcribe(context.Background(), []byte("RIFF"), "meeting.wav")
			require.NoError(t, err)

			assert.Equal(t, tc.wantPath, got.path)
			assert.Equal(t, "Bearer k", got.auth)
			if tc.wantBody != nil {
				assert.Equal(t, tc.wantBody, got.body)
			} else {
				assert.Equal(t, tc.wantFields, got.fields)
				assert.Equal(t, "meeting.wav", got.fileName)
			}
			assert.Equal(t, "hello", out.Text)
			assert.Len(t, out.Segments, tc.wantSegments)
		})
	}
}

// A file over the documented ceiling is refused before it is uploaded: the
// vendor would reject it anyway, after the whole upload.
func TestOversizedAudioIsRefusedBeforeUpload(t *testing.T) {
	url, _, calls := upstream(t)
	a, err := NewASR(&Config{
		Source: types.ModelSourceRemote, Provider: "openai", BaseURL: url + "/v1",
		ModelName: "whisper-1", APIKey: "k",
	})
	require.NoError(t, err)

	_, err = a.Transcribe(context.Background(), make([]byte, 25<<20+1), "long.mp3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 25 MB")
	assert.Zero(t, calls.Load(), "nothing should have been sent")

	_, err = a.Transcribe(context.Background(), make([]byte, 25<<20), "exactly.mp3")
	require.NoError(t, err, "the ceiling itself is allowed")
}

// A vendor that does not declare speech recognition is not trusted with it.
// Azure's Endpoint hook still knows a transcription path, but nobody has
// verified it; a row that names Azure is refused, and a row that names no
// vendor gets what the pre-catalog client sent: the OpenAI shape at its URL.
func TestVendorsWithoutASRAreNotRoutedThroughTheirHooks(t *testing.T) {
	_, err := NewASR(&Config{
		Source: types.ModelSourceRemote, Provider: "azure_openai",
		BaseURL: "https://example.openai.azure.com", ModelName: "whisper", APIKey: "k",
	})
	require.Error(t, err)

	// Detection matches URL patterns as substrings, so a path carrying Azure's
	// host detects as Azure while the request still reaches the test server.
	url, got, _ := upstream(t)
	base := url + "/openai.azure.com/v1"
	require.Equal(t, "azure_openai", modelruntime.DetectByURL(base))
	a, err := NewASR(&Config{
		Source: types.ModelSourceRemote, BaseURL: base, ModelName: "whisper", APIKey: "k",
	})
	require.NoError(t, err)
	_, err = a.Transcribe(context.Background(), []byte("RIFF"), "a.wav")
	require.NoError(t, err)
	assert.Equal(t, "/openai.azure.com/v1/audio/transcriptions", got.path, "not Azure's /openai/v1 path")
	assert.Equal(t, "Bearer k", got.auth, "not Azure's api-key header")
}

// A format the vendor does not list is refused before upload.
func TestUndocumentedFormatIsRefusedBeforeUpload(t *testing.T) {
	url, _, calls := upstream(t)
	a, err := NewASR(&Config{
		Source: types.ModelSourceRemote, Provider: "openai", BaseURL: url + "/v1",
		ModelName: "whisper-1", APIKey: "k",
	})
	require.NoError(t, err)
	_, err = a.Transcribe(context.Background(), []byte("x"), "memo.aac")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flac/mp3")
	assert.Zero(t, calls.Load())
}

// The knowledge base's language hint reaches each vendor where its reference
// puts it, and nowhere for a vendor that documents none.
func TestLanguageHintGoesWhereEachVendorDocumentsIt(t *testing.T) {
	for _, tc := range []struct {
		name, provider, model, base string
		check                       func(t *testing.T, got *form)
	}{
		{"openai form field", "openai", "whisper-1", "/v1", func(t *testing.T, got *form) {
			assert.Equal(t, "zh", got.fields["language"])
			assert.Empty(t, got.language)
		}},
		{
			"litellm documents none", "litellm", "whisper", "/v1",
			func(t *testing.T, got *form) {
				assert.NotContains(t, got.fields, "language")
				assert.Empty(t, got.language)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			url, got, _ := upstream(t)
			a, err := NewASR(&Config{
				Source: types.ModelSourceRemote, Provider: tc.provider, BaseURL: url + tc.base,
				ModelName: tc.model, APIKey: "k",
			})
			require.NoError(t, err)
			_, err = a.Transcribe(WithLanguage(context.Background(), "zh"), []byte("RIFF"), "a.wav")
			require.NoError(t, err)
			tc.check(t, got)
		})
	}

	// "auto" is every vendor's default and is not sent.
	url, got, _ := upstream(t)
	a, err := NewASR(&Config{
		Source: types.ModelSourceRemote, Provider: "openai", BaseURL: url + "/v1", ModelName: "whisper-1", APIKey: "k",
	})
	require.NoError(t, err)
	_, err = a.Transcribe(WithLanguage(context.Background(), "auto"), []byte("RIFF"), "a.wav")
	require.NoError(t, err)
	assert.NotContains(t, got.fields, "language")
}

func TestEmptyAudioIsAnError(t *testing.T) {
	url, _, calls := upstream(t)
	a, err := NewASR(&Config{Source: types.ModelSourceRemote, Provider: "generic", BaseURL: url, ModelName: "m"})
	require.NoError(t, err)
	_, err = a.Transcribe(context.Background(), nil, "a.wav")
	assert.Error(t, err)
	assert.Zero(t, calls.Load())
}

func TestInternalBaseURLIsRefused(t *testing.T) {
	t.Setenv("SSRF_WHITELIST", "")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	_, err := NewASR(&Config{
		Source: types.ModelSourceRemote, Provider: "generic",
		BaseURL: "http://169.254.169.254/latest/meta-data/", ModelName: "asr-test",
	})
	assert.Error(t, err)
}
