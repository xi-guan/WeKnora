package handler

import (
	"github.com/Tencent/WeKnora/internal/models/providers"
	modelruntime "github.com/Tencent/WeKnora/internal/models/runtime"
)

// clouds unreachable from the markets this fork serves, so the editor never offers them
var hiddenVendors = map[string]bool{
	providers.AliyunID:      true,
	providers.HunyuanID:     true,
	providers.LongcatID:     true,
	providers.MimoID:        true,
	providers.MinimaxID:     true,
	providers.ModelscopeID:  true,
	providers.MoonshotID:    true,
	providers.QianfanID:     true,
	providers.QiniuID:       true,
	providers.SiliconflowID: true,
	providers.VolcengineID:  true,
	providers.ZhipuID:       true,
}

// filters the vendor list served to the editor; the registry keeps every vendor so existing rows still resolve
func visibleVendors(in []*modelruntime.Provider) []*modelruntime.Provider {
	out := make([]*modelruntime.Provider, 0, len(in))
	for _, v := range in {
		if hiddenVendors[v.ID] {
			continue
		}
		out = append(out, v)
	}
	return out
}
