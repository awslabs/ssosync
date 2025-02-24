// Copyright (c) 2020, Amazon.com, Inc.
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

// Package internal ...
package internal

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
)

// toPrettyJSON return a json pretty of the stc
func toPrettyJSON(stc interface{}) string {
	JSON, err := json.MarshalIndent(stc, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	return string(JSON)
}

// toJSON return a json of the stc
func toJSON(stc interface{}) string {
	JSON, err := json.Marshal(stc)
	if err != nil {
		log.Fatal(err)
	}
	return string(JSON)
}
