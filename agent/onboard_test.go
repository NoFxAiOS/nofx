package agent

import "testing"

func TestIsDirectSetupCommand(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{text: "setup", want: true},
		{text: "/setup", want: true},
		{text: "开始配置", want: true},
		{text: "配置", want: true},
		{text: "开始设置", want: true},
		{text: "/开始配置", want: false},
		{text: "创建全新的配置，杠杆你定", want: false},
		{text: "帮我配置一个 deepseek 模型", want: false},
		{text: "绑定交易所 okx", want: false},
	}

	for _, tc := range cases {
		if got := isDirectSetupCommand(tc.text); got != tc.want {
			t.Fatalf("isDirectSetupCommand(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
