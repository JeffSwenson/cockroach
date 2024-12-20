// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package main

var test_data_driven_template = `// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

// Code generated by pkg/ccl/backupccl/testgen, DO NOT EDIT.

package backupccl

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

{{- $tests := .TestCases -}}
{{- range $tests }}

func TestDataDriven_{{.TestName}}(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	skip.UnderRace(t, "takes ~3mins to run")
	skip.UnderDeadlock(t, "slows down test by 10 to 100x")

	runTestDataDriven(t, "{{.TestFilePath}}")
}
{{- end }}
`
