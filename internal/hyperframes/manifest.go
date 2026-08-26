package hyperframes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestFile 是上游技能来源记录的文件名，落在 .agents/skills/ 下。
const ManifestFile = "hyperframes-upstream.json"

// ManifestSchema 标识记录格式。
const ManifestSchema = "hyperframes-upstream/v1"

// Manifest 记录一次安装到底装进了哪一版上游技能。
//
// PinnedVersion 固定的只是 CLI 二进制：上游安装器 git clone 仓库默认分支取
// skills/，没有指定 ref 的口子，所以同一个版本号在不同日期装出来的技能可以
// 是不同的。这份记录把"装到了什么"变成可比对的事实——两个项目的 manifest
// 一比就知道差在哪个技能上，而不是只能凭版本号猜。
//
// 只覆盖上游技能：随二进制下发的内置技能因重名被跳过（Result.Protected），
// 它们的内容由仓库自身管，不属于上游来源问题。
type Manifest struct {
	Schema string `json:"schema"`
	// CLIVersion 是安装时使用的上游 CLI 版本，即 PinnedVersion。
	CLIVersion string `json:"cliVersion"`
	// Skills 是技能名到该技能目录摘要的映射。
	Skills map[string]string `json:"skills"`
}

// SkillDigest 计算一个技能目录的内容摘要。
//
// 路径与内容都进哈希，且路径按字典序喂入：只哈希内容的话，两个文件互换
// 文件名会得到相同摘要；不排序的话，同一棵树在不同文件系统上遍历顺序不同，
// 摘要会随机变化，比对就失去意义。
func SkillDigest(dir string) (string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("遍历技能目录 %s 失败：%w", dir, err)
	}
	sort.Strings(paths)

	sum := sha256.New()
	for _, rel := range paths {
		if _, err := io.WriteString(sum, rel+"\x00"); err != nil {
			return "", err
		}
		body, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return "", fmt.Errorf("读取 %s 失败：%w", rel, err)
		}
		if _, err := sum.Write(body); err != nil {
			return "", err
		}
		if _, err := io.WriteString(sum, "\x00"); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// WriteManifest 把本次安装的上游技能来源记录写进项目。
func WriteManifest(projectDir, cliVersion string, installed []string) error {
	skillsDir := filepath.Join(projectDir, filepath.FromSlash(SkillsSubdir))
	m := Manifest{Schema: ManifestSchema, CLIVersion: cliVersion, Skills: map[string]string{}}
	for _, name := range installed {
		digest, err := SkillDigest(filepath.Join(skillsDir, name))
		if err != nil {
			return err
		}
		m.Skills[name] = digest
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 %s 失败：%w", ManifestFile, err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, ManifestFile), append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("写入 %s 失败：%w", ManifestFile, err)
	}
	return nil
}

// ReadManifest 读回项目里的上游技能来源记录。
func ReadManifest(projectDir string) (Manifest, error) {
	path := filepath.Join(projectDir, filepath.FromSlash(SkillsSubdir), ManifestFile)
	body, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("读取 %s 失败：%w", ManifestFile, err)
	}
	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return Manifest{}, fmt.Errorf("解析 %s 失败：%w", ManifestFile, err)
	}
	if !strings.EqualFold(m.Schema, ManifestSchema) {
		return Manifest{}, fmt.Errorf("%s 的 schema 是 %q，期望 %q", ManifestFile, m.Schema, ManifestSchema)
	}
	return m, nil
}
