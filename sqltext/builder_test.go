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

package sqltext

import "testing"

func TestBuilderString(t *testing.T) {
	tests := []struct {
		name  string
		build func(Builder)
		want  string
	}{
		{
			name: "select with joins and grouping",
			build: func(sql Builder) {
				sql.Select("user_name")
				sql.Select("login_name", "creator,create_time")
				sql.From("user u")
				sql.InnerJoin("tenant t ON u.main_id = t.id")
				sql.InnerJoin("tenant2 t2 ON u.main_id = t2.id")
				sql.RightOuterJoin("test t on t.id = u.id")
				sql.Where("u.id = #{id}")
				sql.Where("u.update > 3")
				sql.Or()
				sql.Where("u.login_name=#{login_name}")
				sql.Where("u.age < 3")
				sql.And()
				sql.Where("h=3")
				sql.GroupBy("u.id,u.name")
				sql.Having("count(id) > 0")
				sql.Where("t.=3")
				sql.OrderBy("id,name")
				sql.Limit("10")
				sql.Offset("2")
			},
			want: "SELECT user_name, login_name, creator,create_time\n" +
				"FROM user u\n" +
				"INNER JOIN tenant t ON u.main_id = t.id INNER JOIN tenant2 t2 ON u.main_id = t2.id\n" +
				"RIGHT OUTER JOIN test t on t.id = u.id\n" +
				"WHERE (u.id = #{id} AND u.update > 3) OR (u.login_name=#{login_name} AND u.age < 3) AND (h=3 AND t.=3)\n" +
				"GROUP BY u.id,u.name\n" +
				"HAVING (count(id) > 0)\n" +
				"ORDER BY id,name LIMIT 10 OFFSET 2",
		},
		{
			name: "insert values",
			build: func(sql Builder) {
				sql.InsertInto("user")
				sql.Values("ID, FIRST_NAME", "#{id}, #{firstName}")
				sql.Values("LAST_NAME", "#{lastName}")
				sql.IntoColumns("col1")
				sql.IntoValues("val3")
			},
			want: "INSERT INTO user\n" +
				" (ID, FIRST_NAME, LAST_NAME, col1)\n" +
				"VALUES (#{id}, #{firstName}, #{lastName}, val3)",
		},
		{
			name: "insert multi rows",
			build: func(sql Builder) {
				sql.InsertInto("user")
				sql.IntoColumns("ID, FIRST_NAME", "LAST_NAME")
				sql.IntoValues("#value#", "33", "44")
				sql.AddRow()
				sql.IntoValues("#value#", "33", "44")
			},
			want: "INSERT INTO user\n" +
				" (ID, FIRST_NAME, LAST_NAME)\n" +
				"VALUES (#value#, 33, 44)\n" +
				", (#value#, 33, 44)",
		},
		{
			name: "update",
			build: func(sql Builder) {
				sql.Update("user a")
				sql.Set("a.username = #{name}")
				sql.Set("a.user = 3434")
				sql.Where("a.user_id = 34")
			},
			want: "UPDATE user a\n" +
				"SET a.username = #{name}, a.user = 3434\n" +
				"WHERE (a.user_id = 34)",
		},
		{
			name: "delete",
			build: func(sql Builder) {
				sql.DeleteFrom("ab")
				sql.Where("a.user_id = 34")
				sql.Or()
				sql.Where("ddf = 3")
			},
			want: "DELETE FROM ab\n" +
				"WHERE (a.user_id = 34) OR (ddf = 3)",
		},
		{
			name: "select distinct",
			build: func(sql Builder) {
				sql.SelectDistinct("id", "name")
				sql.From("users")
			},
			want: "SELECT DISTINCT id, name\nFROM users",
		},
		{
			name: "fetch first rows only",
			build: func(sql Builder) {
				sql.Select("id")
				sql.From("users")
				sql.OffsetRows("5")
				sql.FetchFirstRowsOnly("10")
			},
			want: "SELECT id\nFROM users OFFSET 5 ROWS FETCH FIRST 10 ROWS ONLY",
		},
		{
			name: "left outer join",
			build: func(sql Builder) {
				sql.Select("a.id", "b.name")
				sql.From("users a")
				sql.LeftOuterJoin("orders b ON a.id = b.user_id")
				sql.Where("a.status = 1")
			},
			want: "SELECT a.id, b.name\n" +
				"FROM users a\n" +
				"LEFT OUTER JOIN orders b ON a.id = b.user_id\n" +
				"WHERE (a.status = 1)",
		},
		{
			name: "insert returning",
			build: func(sql Builder) {
				sql.InsertInto("users")
				sql.IntoColumns("name")
				sql.IntoValues("#{name}")
				sql.Returning("id")
			},
			want: "INSERT INTO users\n" +
				" (name)\n" +
				"VALUES (#{name})\n" +
				"RETURNING id",
		},
		{
			name: "and or without where are no-ops",
			build: func(sql Builder) {
				sql.Select("id")
				sql.From("users")
				sql.Or()
				sql.And()
			},
			want: "SELECT id\nFROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := New()
			tt.build(sql)

			if got := sql.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
