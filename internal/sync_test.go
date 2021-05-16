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
	"reflect"
	"testing"

	"github.com/awslabs/ssosync/internal/aws"
	log "github.com/sirupsen/logrus"
	admin "google.golang.org/api/admin/directory/v1"
)

// toJSON return a json pretty of the stc
func toJSON(stc interface{}) []byte {
	JSON, err := json.MarshalIndent(stc, "", "  ")
	if err != nil {
		log.Fatalf(err.Error())
	}
	return JSON
}

func Test_getGroupOperations(t *testing.T) {
	type args struct {
		awsGroups    []*aws.Group
		googleGroups []*admin.Group
	}
	tests := []struct {
		name       string
		args       args
		wantAdd    []*aws.Group
		wantDelete []*aws.Group
		wantEquals []*aws.Group
	}{
		{
			name: "equal groups google and aws",
			args: args{
				awsGroups: []*aws.Group{
					aws.NewGroup("Group-1"),
					aws.NewGroup("Group-2"),
				},
				googleGroups: []*admin.Group{
					{Name: "Group-1"},
					{Name: "Group-2"},
				},
			},
			wantAdd:    nil,
			wantDelete: nil,
			wantEquals: []*aws.Group{
				aws.NewGroup("Group-1"),
				aws.NewGroup("Group-2"),
			},
		},
		{
			name: "add two new aws groups",
			args: args{
				awsGroups: nil,
				googleGroups: []*admin.Group{
					{Name: "Group-1"},
					{Name: "Group-2"},
				},
			},
			wantAdd: []*aws.Group{
				aws.NewGroup("Group-1"),
				aws.NewGroup("Group-2"),
			},
			wantDelete: nil,
			wantEquals: nil,
		},
		{
			name: "delete two aws groups",
			args: args{
				awsGroups: []*aws.Group{
					aws.NewGroup("Group-1"),
					aws.NewGroup("Group-2"),
				}, googleGroups: nil,
			},
			wantAdd: nil,
			wantDelete: []*aws.Group{
				aws.NewGroup("Group-1"),
				aws.NewGroup("Group-2"),
			},
			wantEquals: nil,
		},
		{
			name: "add one, delete one and one equal",
			args: args{
				awsGroups: []*aws.Group{
					aws.NewGroup("Group-2"),
					aws.NewGroup("Group-3"),
				},
				googleGroups: []*admin.Group{
					{Name: "Group-1"},
					{Name: "Group-2"},
				},
			},
			wantAdd: []*aws.Group{
				aws.NewGroup("Group-1"),
			},
			wantDelete: []*aws.Group{
				aws.NewGroup("Group-3"),
			},
			wantEquals: []*aws.Group{
				aws.NewGroup("Group-2"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdd, gotDelete, gotEquals := getGroupOperations(tt.args.awsGroups, tt.args.googleGroups)
			if !reflect.DeepEqual(gotAdd, tt.wantAdd) {
				t.Errorf("getGroupOperations() gotAdd = %s, want %s", toJSON(gotAdd), toJSON(tt.wantAdd))
			}
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("getGroupOperations() gotDelete = %s, want %s", toJSON(gotDelete), toJSON(tt.wantDelete))
			}
			if !reflect.DeepEqual(gotEquals, tt.wantEquals) {
				t.Errorf("getGroupOperations() gotEquals = %s, want %s", toJSON(gotEquals), toJSON(tt.wantEquals))
			}
		})
	}
}

func Test_getUserOperations(t *testing.T) {
	type args struct {
		awsUsers    []*aws.User
		googleUsers []*admin.User
	}
	tests := []struct {
		name       string
		args       args
		wantAdd    []*aws.User
		wantDelete []*aws.User
		wantUpdate []*aws.User
		wantEquals []*aws.User
	}{
		{
			name: "equal user google and aws",
			args: args{
				awsUsers: []*aws.User{
					aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
				},
				googleUsers: []*admin.User{
					{Name: &admin.UserName{
						GivenName:  "name-1",
						FamilyName: "lastname-1",
					},
						Suspended:    false,
						PrimaryEmail: "user-1@email.com",
					},
					{Name: &admin.UserName{
						GivenName:  "name-2",
						FamilyName: "lastname-2",
					},
						Suspended:    false,
						PrimaryEmail: "user-2@email.com",
					},
				},
			},
			wantAdd:    nil,
			wantDelete: nil,
			wantUpdate: nil,
			wantEquals: []*aws.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
		},
		{
			name: "add two new aws users",
			args: args{
				awsUsers: nil,
				googleUsers: []*admin.User{
					{Name: &admin.UserName{
						GivenName:  "name-1",
						FamilyName: "lastname-1",
					},
						Suspended:    false,
						PrimaryEmail: "user-1@email.com",
					},
					{Name: &admin.UserName{
						GivenName:  "name-2",
						FamilyName: "lastname-2",
					},
						Suspended:    false,
						PrimaryEmail: "user-2@email.com",
					},
				},
			},
			wantAdd: []*aws.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
			wantDelete: nil,
			wantUpdate: nil,
			wantEquals: nil,
		},
		{
			name: "delete two aws users",
			args: args{
				awsUsers: []*aws.User{
					aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
				},
				googleUsers: nil,
			},
			wantAdd: nil,
			wantDelete: []*aws.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
			wantUpdate: nil,
			wantEquals: nil,
		},
		{
			name: "add on, delete one, update one and one equal",
			args: args{
				awsUsers: []*aws.User{
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
					aws.NewUser("name-3", "lastname-3", "user-3@email.com", true),
					aws.NewUser("name-4", "lastname-4", "user-4@email.com", true),
				},
				googleUsers: []*admin.User{
					{
						Name: &admin.UserName{
							GivenName:  "name-1",
							FamilyName: "lastname-1",
						},
						Suspended:    false,
						PrimaryEmail: "user-1@email.com",
					},
					{
						Name: &admin.UserName{
							GivenName:  "name-2",
							FamilyName: "lastname-2",
						},
						Suspended:    false,
						PrimaryEmail: "user-2@email.com",
					},
					{
						Name: &admin.UserName{
							GivenName:  "name-4",
							FamilyName: "lastname-4",
						},
						Suspended:    true,
						PrimaryEmail: "user-4@email.com",
					},
				},
			},
			wantAdd: []*aws.User{
				aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
			},
			wantDelete: []*aws.User{
				aws.NewUser("name-3", "lastname-3", "user-3@email.com", true),
			},
			wantUpdate: []*aws.User{
				aws.NewUser("name-4", "lastname-4", "user-4@email.com", false),
			},
			wantEquals: []*aws.User{
				aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAdd, gotDelete, gotUpdate, gotEquals := getUserOperations(tt.args.awsUsers, tt.args.googleUsers)
			if !reflect.DeepEqual(gotAdd, tt.wantAdd) {
				t.Errorf("getUserOperations() gotAdd = %s, want %s", toJSON(gotAdd), toJSON(tt.wantAdd))
			}
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("getUserOperations() gotDelete = %s, want %s", toJSON(gotDelete), toJSON(tt.wantDelete))
			}
			if !reflect.DeepEqual(gotUpdate, tt.wantUpdate) {
				t.Errorf("getUserOperations() gotUpdate = %s, want %s", toJSON(gotUpdate), toJSON(tt.wantUpdate))
			}
			if !reflect.DeepEqual(gotEquals, tt.wantEquals) {
				t.Errorf("getUserOperations() gotEquals = %s, want %s", toJSON(gotEquals), toJSON(tt.wantEquals))
			}
		})
	}
}

func Test_getGroupUsersOperations(t *testing.T) {
	type args struct {
		gGroupsUsers   map[string][]*admin.User
		awsGroupsUsers map[string][]*aws.User
	}
	tests := []struct {
		name       string
		args       args
		wantDelete map[string][]*aws.User
		wantEquals map[string][]*aws.User
	}{
		{
			name: "one add, one delete, one equal",
			args: args{
				gGroupsUsers: map[string][]*admin.User{
					"group-1": {
						{
							Name: &admin.UserName{
								GivenName:  "name-1",
								FamilyName: "lastname-1",
							},
							Suspended:    false,
							PrimaryEmail: "user-1@email.com",
						},
					},
				},
				awsGroupsUsers: map[string][]*aws.User{
					"group-1": {
						aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
						aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
					},
				},
			},
			wantDelete: map[string][]*aws.User{
				"group-1": {
					aws.NewUser("name-2", "lastname-2", "user-2@email.com", true),
				},
			},
			wantEquals: map[string][]*aws.User{
				"group-1": {
					aws.NewUser("name-1", "lastname-1", "user-1@email.com", true),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDelete, gotEquals := getGroupUsersOperations(tt.args.gGroupsUsers, tt.args.awsGroupsUsers)
			if !reflect.DeepEqual(gotDelete, tt.wantDelete) {
				t.Errorf("getGroupUsersOperations() gotDelete = %s, want %s", toJSON(gotDelete), toJSON(tt.wantDelete))
			}
			if !reflect.DeepEqual(gotEquals, tt.wantEquals) {
				t.Errorf("getGroupUsersOperations() gotEquals = %s, want %s", toJSON(gotEquals), toJSON(tt.wantEquals))
			}
		})
	}
}
