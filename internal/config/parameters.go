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

// Package config ...
package config

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// Parameters ...
type Parameters struct {
	svc *ssm.SSM
}

// NewParameters ...
func NewParameters(svc *ssm.SSM) *Parameters {
	return &Parameters{
		svc: svc,
	}
}

// UserMappingTemplate ...
func (p *Parameters) UserMappingTemplate(parameterName string) (string, error) {
	if len([]rune(parameterName)) == 0 {
		return p.getParameter("UserMappingTemplate")
	}
	return p.getParameter(parameterName)
}

func (p *Parameters) getParameter(parameterName string) (string, error) {
	r, err := p.svc.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(parameterName),
	})

	if err != nil {
		return "", err
	}

	return *r.Parameter.Value, nil
}
