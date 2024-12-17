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
	"reflect"
	"testing"

	"github.com/awslabs/ssosync/internal/aws"
	"github.com/stretchr/testify/assert"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestMapUser(t *testing.T) {
	type args struct {
		googleUser *admin.User
		template   string
	}
	tests := []struct {
		name     string
		args     args
		wantUser *aws.User
	}{
		{
			name: "Map user attributes",
			args: args{
				googleUser: &admin.User{
					Id:           "701984",
					PrimaryEmail: "test.user@example.com",
					Name: &admin.UserName{
						FamilyName: "a",
						GivenName:  "b",
					},
					Emails: []admin.UserEmail{
						{Address: "test.user@example.com", Type: "work", Primary: true},
						{Address: "test.user1@example.com", Type: "work", Primary: false},
						{Address: "test.user2@example.com", Type: "work", Primary: false},
					},
					Addresses: []admin.UserAddress{
						{
							Type:          "work",
							StreetAddress: "100 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "100 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       true,
						},
						{
							Type:          "home",
							StreetAddress: "101 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "101 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       false,
						},
					},
					Organizations: []admin.UserOrganization{
						{
							Name: "Universal Studios",
							CostCenter: "4130",
							Department: "Tour Operations",
							Domain: "Theme Park",
						},
					},
					Suspended: false,
				},
				template: "",
			},
			wantUser: &aws.User{
				ExternalID: "701984",
				Username:   "test.user@example.com",
				Name: aws.UserName{
					FamilyName: "a",
					GivenName:  "b",
				},
				DisplayName: "b a",
				Emails: []aws.UserEmail{
					{
						Value:   "test.user@example.com",
						Type:    "work",
						Primary: true,
					},
				},
				Addresses: []aws.UserAddress{
					{
						Type:          "work",
						StreetAddress: "100 Universal City Plaza",
						Locality:      "Hollywood",
						Region:        "CA",
						PostalCode:    "91608",
						Country:       "USA",
						Formatted:     "100 Universal City Plaza Hollywood, CA 91608 USA",
						Primary:       true,
					},
				},
				Enterprise: &aws.EnterpriseUser{
					EmployeeNumber: "701984",
					Organization: "Universal Studios",
					CostCenter: "4130",
					Department: "Tour Operations",
					Division: "Theme Park",
				},
				Active:  true,
				Schemas: []string{
					"urn:ietf:params:scim:schemas:core:2.0:User",
					"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User",
				},
			},
		},
		{
			name: "Override user nickname",
			args: args{
				googleUser: &admin.User{
					PrimaryEmail: "test.user@example.com",
					Name: &admin.UserName{
						FamilyName: "a",
						GivenName:  "b",
					},
					Suspended: false,
				},
				template: `{"nickname": {{ splitList "@" .PrimaryEmail | initial | join "" | replace "." "" | quote }}}`,
			},
			wantUser: &aws.User{
				Username: "test.user@example.com",
				Name: aws.UserName{
					FamilyName: "a",
					GivenName:  "b",
				},
				DisplayName: "b a",
				Nickname:    "testuser",
				Active:      true,
				Schemas:     []string{
					"urn:ietf:params:scim:schemas:core:2.0:User",
				},
			},
		},
		{
			name: "Override user addresses with null",
			args: args{
				googleUser: &admin.User{
					PrimaryEmail: "test.user@example.com",
					Name: &admin.UserName{
						FamilyName: "a",
						GivenName:  "b",
					},
					Suspended: false,
					Addresses: []admin.UserAddress{
						{
							Type:          "work",
							StreetAddress: "100 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "100 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       true,
						},
					},
				},
				template: `{"addresses": null}`,
			},
			wantUser: &aws.User{
				Username: "test.user@example.com",
				Name: aws.UserName{
					FamilyName: "a",
					GivenName:  "b",
				},
				DisplayName: "b a",
				Active:      true,
				Addresses:   nil,
				Schemas:     []string{
					"urn:ietf:params:scim:schemas:core:2.0:User",
				},
			},
		},
		{
			name: "Override user addresses with dummy address",
			args: args{
				googleUser: &admin.User{
					PrimaryEmail: "test.user@example.com",
					Name: &admin.UserName{
						FamilyName: "a",
						GivenName:  "b",
					},
					Suspended: false,
					Addresses: []admin.UserAddress{
						{
							Type:          "work",
							StreetAddress: "100 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "100 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       true,
						},
						{
							Type:          "home",
							StreetAddress: "101 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "101 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       false,
						},
					},
				},
				template: `{"addresses": [{"type": "work"}]}`,
			},
			wantUser: &aws.User{
				Username: "test.user@example.com",
				Name: aws.UserName{
					FamilyName: "a",
					GivenName:  "b",
				},
				DisplayName: "b a",
				Active:      true,
				Addresses:   []aws.UserAddress{
					{
						Type: "work",	
					},
				},
				Schemas:     []string{
					"urn:ietf:params:scim:schemas:core:2.0:User",
				},
			},
		},
		{
			name: "Override user addresses with helper method",
			args: args{
				googleUser: &admin.User{
					PrimaryEmail: "test.user@example.com",
					Name: &admin.UserName{
						FamilyName: "a",
						GivenName:  "b",
					},
					Suspended: false,
					Addresses: []admin.UserAddress{
						{
							Type:          "work",
							StreetAddress: "100 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "100 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       true,
						},
						{
							Type:          "home",
							StreetAddress: "101 Universal City Plaza",
							Locality:      "Hollywood",
							Region:        "CA",
							PostalCode:    "91608",
							Country:       "USA",
							Formatted:     "101 Universal City Plaza Hollywood, CA 91608 USA",
							Primary:       false,
						},
					},
				},
				template: `{
					{{- with listFindFirst .Addresses (dict "Type" "home") -}}
					"addresses": [{
						{{- with .StreetAddress -}}"streetAddress": {{ . | quote }},{{- end -}}
						"type": {{ .Type | quote }},
						"primary": true
					}]
					{{- end -}}
				}
				`,
			},
			wantUser: &aws.User{
				Username: "test.user@example.com",
				Name: aws.UserName{
					FamilyName: "a",
					GivenName:  "b",
				},
				DisplayName: "b a",
				Active:      true,
				Addresses:   []aws.UserAddress{
					{
						Type: "home",
						StreetAddress: "101 Universal City Plaza",
						Primary: true,
					},
				},
				Schemas:     []string{
					"urn:ietf:params:scim:schemas:core:2.0:User",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapper, err := NewMapper(tt.args.template)
			assert.NoError(t, err)

			gotUser, err := mapper.Map(&aws.User{}, tt.args.googleUser)
			assert.NoError(t, err)

			if !reflect.DeepEqual(gotUser, tt.wantUser) {
				t.Errorf("Map() gotUser = %s, want %s", toJSON(gotUser), toJSON(tt.wantUser))
			}
		})
	}
}
