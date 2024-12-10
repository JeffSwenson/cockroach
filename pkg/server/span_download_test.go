// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package server

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestMaybeFilterSpans(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	var spans []roachpb.Span
	for i := 0; i < 10; i++ {
		spans = append(spans, roachpb.Span{
			Key: []byte(fmt.Sprintf("span-%d", i)),
		})
	}

	filterSpans := func(nodeID int, fraction float64) []roachpb.Span {
		settings := cluster.MakeTestingClusterSettings()
		testingSkipSpanDownload.Override(ctx, &settings.SV, fraction)
		return maybeFilterSpans(ctx, spans, settings, roachpb.NodeID(nodeID))
	}

	require.Len(t, filterSpans(1, 0.0), 10)
	require.Empty(t, filterSpans(1, 1.0))

	// NOTE: The hash is random, so it's somewhat lucky that it actually splits
	// this case 50/50. But it's deterministic, so it will always have an even
	// split for spans generated by this test.
	require.Len(t, filterSpans(1, 0.5), 5)

	// The node id is included in the hash
	require.Equal(t, filterSpans(1, 0.5), filterSpans(1, 0.5))
	require.NotEqual(t, filterSpans(1, 0.5), filterSpans(2, 0.5))
}
