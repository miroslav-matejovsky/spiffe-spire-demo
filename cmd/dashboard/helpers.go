package main

import (
	"fmt"
	"strings"
	"time"

	apitypes "github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

func spiffeIDStr(id *apitypes.SPIFFEID) string {
	if id == nil {
		return ""
	}
	return fmt.Sprintf("spiffe://%s%s", id.TrustDomain, id.Path)
}

func selectorsStr(sels []*apitypes.Selector) string {
	parts := make([]string, 0, len(sels))
	for _, s := range sels {
		parts = append(parts, s.Type+":"+s.Value)
	}
	return strings.Join(parts, ", ")
}

func formatUnixTime(ts int64) string {
	if ts == 0 {
		return "-"
	}
	return time.Unix(ts, 0).UTC().Format("2006-01-02 15:04:05 UTC")
}
