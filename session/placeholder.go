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

// getPlaceholders extracts #{...} and ${...} placeholders from s.
func getPlaceholders(s string) []string {
	sIndex := -1
	placeholders := make([]string, 0)
	bytes := []byte(s)
	for i, v := range bytes {
		if (v == '#' || v == '$') && i+1 < len(bytes) && bytes[i+1] == '{' {
			sIndex = i
		} else if v == '}' && sIndex != -1 {
			placeholders = append(placeholders, s[sIndex:i+1])
			sIndex = -1
		}
	}
	return placeholders
}

// splitPlaceholders separates #{...} (dynamic / parameterized) from ${...} (injected / literal).
func splitPlaceholders(s string) (dynamic, injected []string) {
	dynamic = make([]string, 0)
	injected = make([]string, 0)
	var isDynamic bool
	sIndex := -1
	bytes := []byte(s)
	for i, v := range bytes {
		if v == '#' && i+1 < len(bytes) && bytes[i+1] == '{' {
			sIndex = i
			isDynamic = true
		} else if v == '$' && i+1 < len(bytes) && bytes[i+1] == '{' {
			sIndex = i
			isDynamic = false
		} else if v == '}' && sIndex != -1 {
			if isDynamic {
				dynamic = append(dynamic, s[sIndex:i+1])
			} else {
				injected = append(injected, s[sIndex:i+1])
			}
			sIndex = -1
		}
	}
	return
}
