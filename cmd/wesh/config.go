// TOML 配置文件加载层（OPS-09，07-06，D-01..D-07，07-RESEARCH Pattern 4 配方）。
//
// 决策依据（07-CONTEXT 锁定，one-way 公开契约）：
//   - D-01：路径发现 = 仅 --config 显式指定，零隐式默认路径搜索——裸 wesh 行为
//     与无配置文件时完全一致；prescanConfigPath 是装配链第一环（parseArgs 在
//     flag 注册前手工预扫出路径）。
//   - D-03：TOML 形状 = 平铺 key = value，键名 = flag 名——fileConfig 的 toml
//     tag 与 flag 名逐字一致；拒绝分组 sections。
//   - D-04：覆盖面 = 27 个长期运行 flag 同名键、command exec 数组与
//     index-max-size 纯配置键，共 29 键（09-04 D-07 index 随 flag 面；
//     index-max-size 无对应 flag 是 D-08 裁决的明示例外——P7 D-03 纪律的
//     首个纯配置键，防例外蔓延 README 写明）；no-auth/insecure-http/version/
//     help/config 五逃生门键不入 fileConfig——严格模式（未知键拒绝）将其
//     自然拒绝（逃生门必须显式说出口，配置文件里写出来等于没说）。
//   - D-06：加载失败与未知键 = exit 2 fail-fast 严格模式（文件不存在 / TOML
//     解析失败 / 未知键均拒绝——错误经返回值上抛，exit 2 由调用方 parseArgs
//     现状通道落地）。
//   - D-07：含 credential 键且文件权限非 600/400 → stderr 警告放行（不阻断——
//     挂载盘/容器 secret 权限语义不可靠，ssh 式拒绝误伤多）。
//
// 合并前提：fileConfig 全部标量为指针类型——指针区分「键缺席」（nil）与
// 「显式零值」（非 nil 指向 0/false/""），parseArgs 两阶段合并的正确性依赖
// nil 判定（值拷贝会把显式零值吞成「缺席」，port = 0 随机端口等形态随之漂移）。
//
// 值剥离红线（SEC-01 启动面红线延伸，07-RESEARCH Pitfall 5）：go-toml 的
// DecodeError.String()/Error() 与 StrictMissingError.String() 带源行上下文，
// 会回显配置值（含 credential 明文）——本文件绝不透传 go-toml 错误文本，只提取
// DecodeError.Key()/Position()（键名 + 行号）组 detail，全部错误经 configErr
// 统一包装为「类别 + 键名/行号」三要素。测试以凭据值探针串运行时自证零出现。
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// fileConfig 是配置文件的解码目标：29 键 = 27 flag 同名 + command +
// index-max-size 纯配置键（D-04 覆盖面 27→29，09-04）。
// 标量全指针（键缺席 = nil，见文件头「合并前提」）；列表 nil = 键缺席，
// 非 nil 空数组按缺席语义处理（与 CLI `--` 空 argv 同档，plan flagged_assumptions）。
type fileConfig struct {
	Port          *int     `toml:"port"`
	Bind          *string  `toml:"bind"`
	Writable      *bool    `toml:"writable"`
	PingInterval  *string  `toml:"ping-interval"` // duration 串，合并期 time.ParseDuration 复用
	WritePolicy   *string  `toml:"write-policy"`
	MaxClients    *int     `toml:"max-clients"`
	Once          *bool    `toml:"once"`
	ExitWhenEmpty *string  `toml:"exit-when-empty"` // 字符串单形态（OQ4），合并期 exitEmptyValue.Set 复用；bool 形态由 go-toml 类型不符自然拒绝
	Credential    []string `toml:"credential"`
	Origin        []string `toml:"origin"`
	ClientOption  []string `toml:"client-option"`
	TLSCert       *string  `toml:"tls-cert"`
	TLSKey        *string  `toml:"tls-key"`
	Osc52         *bool    `toml:"osc52"`
	Socket        *string  `toml:"socket"`
	SocketMode    *string  `toml:"socket-mode"` // 八进制串，合并期 strconv.ParseUint 复用
	SocketOwner   *string  `toml:"socket-owner"`
	BasePath      *string  `toml:"base-path"`
	AuthHeader    *string  `toml:"auth-header"`
	Cwd           *string  `toml:"cwd"`
	Term          *string  `toml:"term"`
	StopSignal    *string  `toml:"stop-signal"`
	StopTimeout   *string  `toml:"stop-timeout"` // duration 串，合并期 time.ParseDuration 复用
	Uid           *int     `toml:"uid"`
	Gid           *int     `toml:"gid"`
	Open          *bool    `toml:"open"`
	Command       []string `toml:"command"` // D-04：exec 数组；CLI `--` 后 argv 非空则覆盖
	// 09-04 D-07/D-08（OPS-03 自定义首页）：index 为 --index flag 同名键（配置
	// 铺底、CLI 覆盖）；index-max-size 为整数字节纯配置键（无对应 CLI flag——
	// D-08 裁决，P7 D-03 纪律的明示例外；默认 16MiB 在 main.go parseArgs 铺底，
	// ≤0 经 validateStartup 拒绝）。
	Index        *string `toml:"index"`
	IndexMaxSize *int    `toml:"index-max-size"` // 整数字节（OQ1 推荐形态，max-clients 整数先例同型零新解析；字符串形态由 go-toml 类型不符自然拒绝）
	// D-04 排除项不在结构体：no-auth/insecure-http/version/help/config
	// → 严格模式以「未知键」拒绝（逃生门必须显式说出口）。
}

// configErr 统一包装配置文件错误：`invalid config file <path>: <category>
// (<detail>)`——detail 只含键名/行号/安全类别文案，禁含配置值（文件头红线）。
func configErr(path, category, detail string) error {
	return fmt.Errorf("invalid config file %s: %s (%s)", path, category, detail)
}

// decodeFileConfig 承载 TOML 解码与错误分类（fuzz 接缝，09-02 D-09）：reader
// 委托使 bytes-in 可测（FuzzDecodeFileConfig 经 bytes.NewReader 直驱）；错误
// 分类三分支与 configErr 包装为唯一副本（单写口不复制第二份——值剥离红线逻辑
// 分叉即 SEC-01 破口），path 参数仅供 configErr 错误包装，不从 path 读盘。
//
// 严格模式经 Decoder 链式严格模式方法启用（官方唯一严格模式 API，该库无
// SetStrict 方法——07-RESEARCH 明示，pkg.go.dev CITED；验收机械检查锚定下方
// 调用行）。错误分类：
//   - 未知键 → *toml.StrictMissingError 经 errors.As 提取键名清单组 detail
//     （StrictMissingError.Error() 是固定类别串，但其 String() 与逐个
//     DecodeError.String() 带源行上下文会回显值——只取 Key()，Pitfall 5）；
//   - TOML 解析失败/类型不符 → *toml.DecodeError 提取 Key()/Position() 组
//     detail（Error() 文本对语法错误含源字符 %#U，同样不透传，Pitfall 5）；
//   - 兜底 → cannot parse（非 go-toml 结构化错误不透传原始文本，红线无例外通道）。
func decodeFileConfig(path string, r io.Reader) (*fileConfig, error) {
	var decoded fileConfig
	derr := toml.NewDecoder(r).DisallowUnknownFields().Decode(&decoded)
	if derr != nil {
		var strictErr *toml.StrictMissingError
		if errors.As(derr, &strictErr) {
			keys := make([]string, 0, len(strictErr.Errors))
			for i := range strictErr.Errors {
				keys = append(keys, strings.Join(strictErr.Errors[i].Key(), "."))
			}
			return nil, configErr(path, "unknown keys", strings.Join(keys, ", "))
		}
		var decodeErr *toml.DecodeError
		if errors.As(derr, &decodeErr) {
			row, _ := decodeErr.Position()
			detail := fmt.Sprintf("line %d", row)
			if key := decodeErr.Key(); len(key) > 0 {
				detail = fmt.Sprintf("key %q line %d", strings.Join(key, "."), row)
			}
			return nil, configErr(path, "invalid toml", detail)
		}
		// 兜底：非 go-toml 结构化错误（io 读取失败等）——不透传原始错误文本，
		// 只报类别（值剥离红线无例外通道）。
		return nil, configErr(path, "cannot parse", "unrecognized toml error")
	}
	return &decoded, nil
}

// loadFileConfig 加载并严格校验 TOML 配置文件（D-06 fail-fast 的错误经返回值
// 上抛，exit 2 由调用方落地；NormalizeOrigin 同位纯函数形态——无副作用）。
// 缩为 open-file + 委托 decodeFileConfig（解码/错误分类唯一副本在委托内）+
// D-07 权限警告；错误分类第一分支：
//   - 文件不存在/不可读 → cannot read（OS 错误只含路径与系统类别，非配置值）。
//
// D-07：fc.Credential != nil 且 os.Stat(path).Mode().Perm() 非 0600/0400 →
// 返回 warn 串（警告形态照 validateStartup 的 `wesh: warning:` 前缀先例，
// main.go:691,693,699），警告串不含凭据值；Stat 失败不升级（加载已成功，
// 权限检查是提醒而非闸门）。Stat 仍按 path（委托只持 reader，权限语义属文件）。
func loadFileConfig(path string) (fc *fileConfig, warn string, err error) {
	f, err := os.Open(path) // D-06：不存在即拒（exit 2 由调用方落地）
	if err != nil {
		return nil, "", configErr(path, "cannot read", err.Error())
	}
	defer func() { _ = f.Close() }()
	decoded, derr := decodeFileConfig(path, f)
	if derr != nil {
		return nil, "", derr
	}
	// D-07：含 credential 键且权限非 600/400 → stderr 警告放行（不阻断）。
	if decoded.Credential != nil {
		if info, serr := os.Stat(path); serr == nil {
			perm := info.Mode().Perm()
			if perm != 0o600 && perm != 0o400 {
				warn = fmt.Sprintf("wesh: warning: config file %s contains credentials and is readable by others (mode %04o); recommend chmod 600", path, perm)
			}
		}
	}
	return decoded, warn, nil
}

// prescanConfigPath 手工预扫 --config 路径（D-01 装配前提）：flag 注册前需要
// 路径加载 TOML 铺底，故不能等正式 Parse。扫描 `--config=<v>` 与 `--config <v>`
// 两形态（单横线变体与 flag 包解析语义一致），last-wins；未给返回 ""。
// 扫描器不解析其他 flag、不报错（正式 Parse 是形状校验的唯一闸门——预扫与
// 正式 Parse 的 --config 值一致）；`--` 之后是子命令 argv，其 --config 属于
// 子命令不扫描（绝不为子命令参数加载配置文件）。
func prescanConfigPath(args []string) string {
	path := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			break
		}
		name, val, hasEq := a, "", false
		if j := strings.IndexByte(a, '='); j >= 0 {
			name, val, hasEq = a[:j], a[j+1:], true
		}
		if name != "--config" && name != "-config" {
			continue
		}
		if hasEq {
			path = val
		} else if i+1 < len(args) {
			path = args[i+1]
			i++
		}
	}
	return path
}
