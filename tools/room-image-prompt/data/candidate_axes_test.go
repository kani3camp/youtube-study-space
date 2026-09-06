package data

import (
	"strings"
	"testing"
)

func TestCandidateAxesStaySeparated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		file      string
		forbidden []string
	}{
		{
			file: "01_world.txt",
			forbidden: []string{
				"夜明け", "早朝", "朝", "午前", "昼前", "正午", "昼下がり", "午後",
				"夕方", "夕暮れ", "日没", "宵", "夜", "深夜",
				"静けさ", "ぬくもり", "爽やか", "安心感", "集中感", "清潔感",
				"パステル", "木目", "真鍮", "間接照明",
			},
		},
		{
			file: "02_time_of_day.txt",
			forbidden: []string{
				"春", "夏", "秋", "冬", "雨", "雪", "月夜", "星空",
				"休日", "週末", "静か", "静けさ", "作業", "朝活",
			},
		},
		{
			file: "03_workspace_type.txt",
			forbidden: []string{
				"夜明け", "早朝", "朝活", "朝食", "昼下がり", "午後", "夕方",
				"夕暮れ", "日没", "宵", "深夜", "夜カフェ", "週末",
				"雨の日", "晴れた", "春の", "夏の", "秋の", "冬の", "雪の日",
				"白基調", "北欧風", "無垢材", "白壁", "真鍮", "アンティーク",
				"ヴィンテージ", "大正モダン", "昭和レトロ", "ミニマル", "淡色",
			},
		},
		{
			file: "04_seat_layout.txt",
			forbidden: []string{
				"UI", "カード", "座席番号", "撮影", "安全地帯", "背景低密度", "画面",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()
			lines := readCandidateLines(t, tt.file)
			for _, line := range lines {
				for _, term := range tt.forbidden {
					if strings.Contains(line, term) {
						t.Errorf("%s contains out-of-axis term %q: %q", tt.file, term, line)
					}
				}
			}
		})
	}
}

func TestCandidateFilesHaveNoDuplicateLines(t *testing.T) {
	t.Parallel()

	for _, file := range []string{
		"01_world.txt",
		"02_time_of_day.txt",
		"03_workspace_type.txt",
		"04_seat_layout.txt",
	} {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			seen := make(map[string]bool)
			for _, line := range readCandidateLines(t, file) {
				if seen[line] {
					t.Errorf("%s contains duplicate candidate %q", file, line)
				}
				seen[line] = true
			}
		})
	}
}

func readCandidateLines(t *testing.T, file string) []string {
	t.Helper()

	body, err := FS.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	var lines []string
	for _, raw := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		t.Fatalf("%s has no candidates", file)
	}
	return lines
}
