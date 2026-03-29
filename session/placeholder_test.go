// Copyright 2025 Dennis Ge
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package session

import (
	"reflect"
	"testing"
)

func TestGetPlaceholders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "single dynamic", input: "id = #{id}", want: []string{"#{id}"}},
		{name: "mixed placeholders", input: "col = ${col} AND id = #{id}", want: []string{"${col}", "#{id}"}},
		{name: "no placeholders", input: "no placeholder", want: nil},
		{name: "trailing hash", input: "trailing #", want: nil},
		{name: "trailing dollar", input: "trailing $", want: nil},
		{name: "multiple dynamics", input: "#{a} AND #{b}", want: []string{"#{a}", "#{b}"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeStringSlice(getPlaceholders(tt.input))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("getPlaceholders(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitPlaceholders(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantDynamic  []string
		wantInjected []string
	}{
		{
			name:         "mixed placeholders",
			input:        "col = ${col} AND id = #{id}",
			wantDynamic:  []string{"#{id}"},
			wantInjected: []string{"${col}"},
		},
		{
			name:         "trailing symbol",
			input:        "value = #",
			wantDynamic:  nil,
			wantInjected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dynamic, injected := splitPlaceholders(tt.input)
			dynamic = normalizeStringSlice(dynamic)
			injected = normalizeStringSlice(injected)

			if !reflect.DeepEqual(dynamic, tt.wantDynamic) {
				t.Fatalf("dynamic = %v, want %v", dynamic, tt.wantDynamic)
			}
			if !reflect.DeepEqual(injected, tt.wantInjected) {
				t.Fatalf("injected = %v, want %v", injected, tt.wantInjected)
			}
		})
	}
}

func normalizeStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return values
}
