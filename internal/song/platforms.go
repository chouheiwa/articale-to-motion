package song

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/fsutil"
	"gopkg.in/yaml.v3"
)

type Platform struct{ ID, Name, Model, Key, Console, Docs, Notes string }

var Platforms = []Platform{
	{"manual", "网页手动生成 / 导入歌曲", "external", "", "由用户选择歌曲生成网页", "am song handoff --help", "无需 API Key；输出歌词和曲风说明后暂停，用户在网页生成并提供音频，再运行 am song receive AUDIO。"},
	{"mureka", "Mureka 官方 API", "mureka-9.5", "MUREKA_API_KEY", "https://platform.mureka.ai/", "https://platform.mureka.ai/docs/en/quickstart.html", "支持中文及自带歌词；API 单独充值，与网页会员不互通。每次请求一首，按任务 ID 续跑；时长/BPM 仅作为创作提示。"},
	{"lyria", "Google Lyria", "lyria-3.5", "GEMINI_API_KEY", "https://aistudio.google.com/api-keys", "https://ai.google.dev/gemini-api/docs/music-generation", "需 Gemini API 项目及对应模型权限/计费；使用 Interactions API 指定歌词。目标时长与 BPM 是提示，需试听核对是否改词；同步请求中断不自动重发。"},
	{"bailian", "阿里云百炼 Fun-Music", "fun-music-v1", "DASHSCOPE_API_KEY", "https://bailian.console.aliyun.com/", "https://help.aliyun.com/zh/model-studio/fun-music-api", "需北京地域 Fun-Music 邀测权限及普通按量 API Key；当前 Token Plan 清单未包含音乐。指定歌词时曲风提示被忽略，时长/BPM 不作为 API 控制参数。"},
	{"elevenlabs", "ElevenLabs Music", "music_v2_5", "ELEVENLABS_API_KEY", "https://elevenlabs.io/app/developers/api-keys", "https://elevenlabs.io/docs/api-reference/music/compose", "需有 Music API 权限及可用付费额度；使用固定歌词 composition_plan，曲风建议用英文。"},
	{"fal", "fal 托管 ACE-Step", "fal-ai/ace-step", "FAL_KEY", "https://fal.ai/dashboard/keys", "https://fal.ai/models/fal-ai/ace-step/api", "无需自建服务器；使用 fal 队列协议，不能填到自建 ACE-Step endpoint；不宣称该模型为 ACE-Step 1.5。"},
	{"minimax", "MiniMax（mmx）", "music-3.0", "", "https://www.minimax.io/audio", "https://platform.minimax.io/docs/api-reference/music-generation", "复用 mmx 登录；已实测部分账户返回 HTTP 410（音乐 API 不再向新用户开放）。登录成功不等于音乐权限可用。"},
	{"acestep", "自建/远端 ACE-Step", "acestep-v15-turbo", "ACESTEP_API_KEY", "https://github.com/ace-step/ACE-Step-1.5", "https://github.com/ace-step/ACE-Step-1.5/blob/main/docs/en/API.md", "必须已有可访问服务地址；密钥按服务设置，可为空。不自动部署模型。"},
}

func providerInfo(id string) Platform {
	for _, p := range Platforms {
		if p.ID == id {
			return p
		}
	}
	return Platform{}
}
func knownProvider(id string) bool { return providerInfo(id).ID != "" }
func PlatformGuide(out io.Writer, id string, env map[string]string) error {
	if id != "" && !knownProvider(id) {
		return fmt.Errorf("未知歌曲平台；运行 am song providers 查看支持清单")
	}
	for _, p := range Platforms {
		if id != "" && id != p.ID {
			continue
		}
		if p.ID == "manual" {
			fmt.Fprintf(out, "\n%s [%s]\n  %s\n", p.Name, p.ID, p.Notes)
		} else {
			fmt.Fprintf(out, "\n%s [%s]\n  默认模型：%s\n  %s\n  开通/密钥：%s\n  API 文档：%s\n", p.Name, p.ID, p.Model, p.Notes, p.Console, p.Docs)
		}
		if p.Key != "" {
			status := "未设置"
			if strings.TrimSpace(env[p.Key]) != "" {
				status = "已设置（未验证权限/额度）"
			}
			fmt.Fprintf(out, "  %s：%s\n  在本机终端设置环境变量（不要填入 song.yaml）：\n    export %s='<你的 API Key>'\n  am run 嵌套调用还需显式透传（保留已有列表）：\n    export AM_PASSTHROUGH_ENV=\"${AM_PASSTHROUGH_ENV:+${AM_PASSTHROUGH_ENV},}%s\"\n", p.Key, status, p.Key, p.Key)
		} else if p.ID == "manual" {
			fmt.Fprintln(out, "  写好歌词后运行 am song handoff；上传生成的音频或提供本机路径后继续。")
		} else {
			fmt.Fprintln(out, "  按 mmx auth --help 完成登录。")
		}
		extra := ""
		if p.ID == "bailian" {
			extra = " --workspace YOUR_WORKSPACE_ID"
		}
		if p.ID == "acestep" {
			extra = " --endpoint https://YOUR_SERVER"
		}
		fmt.Fprintf(out, "  配置：am song configure --provider %s%s\n", p.ID, extra)
	}
	if id == "manual" {
		fmt.Fprintln(out, "\n流程：编写歌词/曲风 → am song handoff → 等待用户提供音频 → am song receive AUDIO → 用户确认 → am song select ID → am song prepare。网页模式无需 API Key。")
		return nil
	}
	fmt.Fprintln(out, "\n配置后：am song doctor → 编写 lyrics.txt → am song generate --count 1 → 试听后 am song select <id>。\n配置和密钥检查不会产生生成费用，也不会验证账户额度。外部歌曲可用 am song import 导入。")
	return nil
}

var workspacePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{0,62}$`)

func validateHostedConfig(c Config) error {
	if c.Workspace != "" && c.Provider != "bailian" {
		return fmt.Errorf("workspace 仅适用于百炼；切换平台请用 am song configure")
	}
	switch c.Provider {
	case "bailian", "elevenlabs", "fal", "mureka", "lyria", "manual":
		if c.Endpoint != "" {
			return fmt.Errorf("托管平台不接受自定义 endpoint；am song configure 可清理旧服务地址")
		}
	}
	switch c.Provider {
	case "manual":
		if c.Model != "external" {
			return fmt.Errorf("manual 模式 model 必须为 external，具体网页模型由用户选择")
		}
	case "mureka":
		switch c.Model {
		case "mureka-7.6", "mureka-o2", "mureka-8", "mureka-9", "mureka-9.5":
		default:
			return fmt.Errorf("Mureka 请指定固定模型：mureka-7.6|mureka-o2|mureka-8|mureka-9|mureka-9.5")
		}
	case "lyria":
		if c.Model != "lyria-3.5" {
			return fmt.Errorf("Lyria 整曲模式当前支持 lyria-3.5")
		}
	case "bailian":
		if !workspacePattern.MatchString(c.Workspace) {
			return fmt.Errorf("百炼需要北京业务空间 ID：am song configure --provider bailian --workspace YOUR_WORKSPACE_ID")
		}
		if c.Model != "fun-music-v1" && c.Model != "fun-music-preview" {
			return fmt.Errorf("百炼音乐模型仅支持 fun-music-v1 或 fun-music-preview")
		}
	case "elevenlabs":
		if c.Model != "music_v2" && c.Model != "music_v2_5" {
			return fmt.Errorf("ElevenLabs chunks 接口仅支持 music_v2 或 music_v2_5")
		}
	case "fal":
		if c.Model != "fal-ai/ace-step" {
			return fmt.Errorf("fal 当前适配模型为 fal-ai/ace-step")
		}
	}
	return nil
}
func CheckCredential(c Config, env map[string]string) error {
	p := providerInfo(c.Provider)
	if p.Key == "" || c.Provider == "acestep" {
		return nil
	}
	key := strings.TrimSpace(env[p.Key])
	if key == "" {
		return fmt.Errorf("缺少 %s；运行 am song providers %s 查看申请及配置步骤；am run 需通过 AM_PASSTHROUGH_ENV 透传", p.Key, p.ID)
	}
	if c.Provider == "bailian" && strings.HasPrefix(key, "sk-sp-") {
		return fmt.Errorf("Fun-Music 不支持 Token Plan 专用 Key，请配置北京地域普通 DASHSCOPE_API_KEY")
	}
	return nil
}

// Configure edits only platform fields, preserving user lyrics/style and YAML comments.
// The whole proposed file is validated before the atomic replacement.
func Configure(root, provider, model, workspace, endpoint string) error {
	if !knownProvider(provider) {
		return fmt.Errorf("必须指定 --provider；运行 am song providers 查看支持清单")
	}
	return withLock(root, func() error {
		path, e := LocalPath(root, "song.yaml")
		if e != nil {
			return e
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return fmt.Errorf("请先运行 am song init：%w", e)
		}
		var doc yaml.Node
		decoder := yaml.NewDecoder(bytes.NewReader(b))
		if e = decoder.Decode(&doc); e != nil || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
			return fmt.Errorf("song.yaml 格式无效")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return fmt.Errorf("song.yaml 只能包含一份配置")
		}
		if model == "" {
			model = providerInfo(provider).Model
		}
		replacements := map[string]string{"provider": provider, "model": model, "workspace": workspace, "endpoint": endpoint}
		node := doc.Content[0]
		var fields []*yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			k, v := node.Content[i], node.Content[i+1]
			if value, ok := replacements[k.Value]; ok {
				delete(replacements, k.Value)
				if value == "" {
					continue
				}
				v.Value = value
				v.Tag = "!!str"
				v.Kind = yaml.ScalarNode
			}
			fields = append(fields, k, v)
		}
		node.Content = fields
		for _, k := range []string{"provider", "model", "workspace", "endpoint"} {
			if v := replacements[k]; v != "" {
				node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: k}, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
			}
		}
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if e = enc.Encode(&doc); e != nil {
			return e
		}
		enc.Close()
		// Validate against the real project root (including lyrics path and cast checks).
		_, e = decodeConfig(root, buf.Bytes())
		if e != nil {
			return e
		}
		info, e := os.Stat(filepath.Join(root, "song.yaml"))
		if e != nil {
			return e
		}
		return fsutil.AtomicWrite(path, buf.Bytes(), info.Mode().Perm())
	})
}
