// Copyright 2018-2023 CERN
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
//
// In applying this license, CERN does not waive the privileges and immunities
// granted to it by virtue of its status as an Intergovernmental Organization
// or submit itself to any jurisdiction.

package utils

import "strings"

// clean the url putting a slash (/) at the beginning if it does not have it
// and removing the slashes at the end
// if the url is "/", the output is "".
func cleanURL(url string) string {
	if len(url) > 0 {
		if url[0] != '/' {
			url = "/" + url
		}
		url = strings.TrimRight(url, "/")
	}
	return url
}

func UrlHasPrefix(url, prefix string) bool {
	url = cleanURL(url)
	prefix = cleanURL(prefix)

	partsURL := strings.Split(url, "/")
	partsPrefix := strings.Split(prefix, "/")

	if len(partsPrefix) > len(partsURL) {
		return false
	}

	for i, p := range partsPrefix {
		u := partsURL[i]
		if p != u {
			return false
		}
	}

	return true
}

func GetSubURL(url, prefix string) string {
	// pre cond: prefix is a prefix for url
	// example: url = "/api/v0/", prefix = "/api", res = "/v0"
	url = cleanURL(url)
	prefix = cleanURL(prefix)

	return url[len(prefix):]
}
