// 本文件把 internal/validate.CastProblems 接到 am validate 命令组上，
// 与 newCastCmd 对 internal/cast 的分工一致：internal/validate 只定义
// 项目级一致性规则，这里是唯一把它接到 CLI 上的地方。
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/chouheiwa/articale-to-motion/internal/config"
	"github.com/chouheiwa/articale-to-motion/internal/validate"
	"github.com/spf13/cobra"
)

func newValidateCastCommand(stdout io.Writer) *cobra.Command {
	var projectRoot, provider string
	cmd := &cobra.Command{
		Use:   "cast",
		Args:  noArgs,
		Short: "校验多角色项目的项目级一致性",
		Long: `校验班底、对白时间线与镜头节拍三者是否互相吻合：

  - cast.yaml 登记的角色包能否加载，班底是否为空
  - 每个角色是否声明了当前 TTS_PROVIDER 的音色
  - production/dialogue.json 是否存在、schema 正确，字幕行数与
    transcription-production.srt 是否一致，逐行时间是否连续不重叠
  - dialogue.json 里的说话人是否都在班底里
  - 所有 scenes/*/scene.json 的 cast.beats 换算回全局时间后，是否合并
    覆盖了 dialogue.json 的全部台词行，以及每个 on_stage 角色的 dna.md
    是否存在于渲染提示词会引用的路径

项目没有 cast.yaml（单口播模式）时视为无事可做，直接退出 0：这组检查
只对多角色项目生效，对老项目零影响。

TTS_PROVIDER 缺省按 am config get TTS_PROVIDER 的解析结果，--provider
可显式覆盖。`,
		Example: `  am validate cast
  am validate cast --provider bailian
  am validate cast --project-root .`,
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir := projectRoot
			if rootDir == "" {
				rootDir = "."
			}
			resolvedProvider := provider
			if resolvedProvider == "" {
				cfg, err := config.Load(rootDir, nil)
				if err != nil {
					return err
				}
				resolvedProvider = cfg.TTSProvider
			}
			problems := validate.CastProblems(rootDir, resolvedProvider)
			if len(problems) > 0 {
				return fmt.Errorf("多角色项目一致性校验未通过：\n  - %s", strings.Join(problems, "\n  - "))
			}
			fmt.Fprintln(stdout, "多角色项目一致性校验通过")
			return nil
		},
	}
	cmd.Flags().StringVar(&projectRoot, "project-root", "", "项目根目录，缺省为当前目录")
	cmd.Flags().StringVar(&provider, "provider", "", "TTS_PROVIDER，缺省取 am config 解析结果")
	return cmd
}
