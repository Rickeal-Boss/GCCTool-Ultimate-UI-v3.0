package client

// ─────────────────────────────────────────────────────────────────────────────
// 端点参数白名单（实机抓包结论，2026-09-15，jwxt.gcc.edu.cn）
//
// 核心结论：**表单只提交自己 form 的字段**。
// 同一会话 4 组对照实验（整包参数池 208 项 vs 浏览器白名单）：
//
//	| 组合                          | 结果            |
//	|-------------------------------|-----------------|
//	| A 白名单（浏览器实际提交字段） | flag=1，288 门课 |
//	| B 整个参数池（208 项隐藏字段） | 服务端「系统运行异常」|
//	| C 白名单去掉 kkbm_id_list[0]  | 288 门（非必需）  |
//	| D 参数池 + kkbm_id_list       | 仍「系统运行异常」|
//
// → V3.0/V3.1 的"页面下发什么就整体回填"策略会把登录页/导航等**其他 form**
//   的隐藏字段一起发进选课请求，被服务端拒绝。现在按端点拆白名单：
//     - 课程列表 / 课程详情 / 已选查询 → browserListParams / browserInfoParams
//     - 选课提交                        → selectParamNames（+xkkz_xh）
//     - 退课                            → cancelParamNames（只 5 个字段）
//     - Display（选课开关页）           → displayParamNames（浏览器同款 7 参数）
//
// 名单来源：Efarxs（同校对标实现）抓包的 data.json —— classList(47) /
// classInfo(44)，与浏览器 DevTools 实际请求体一致。
// 注意：名单是"契约"而非"页面全集"；学校换正方版本时只需更新本文件。
// ─────────────────────────────────────────────────────────────────────────────

// browserListParams 课程列表查询（PartDisplay）白名单 —— data.json classList(47)
var browserListParams = []string{
	"xkly", "jg_id", "zyh_id", "sfkkzy", "sfkxq", "kklxdm",
	"jxbzb", "bh_id", "mzm", "xz", "kkbkdj", "gnjkxdnj",
	"xslbdm", "xsbj", "sfkcfx", "bbhzxjxb", "sfkkjyxdxnxq", "zdkxms",
	"zyh_id_xs", "sfkgbcx", "sfrxtgkcxd", "ccdm", "sfkknj", "xkkz_id",
	"rlkz", "zh", "rwlx", "bklx_id", "kzkcgs", "xqh_id",
	"xbm", "kspage", "jspage", "njdm_id_1", "njdm_id", "kzybkxy",
	"njdm_id_xs", "xkxqm", "xkzgbj", "xkxnm", "xklc", "zyh_id_1",
	"zyfx_id", "bjgkczxbbjwcx", "sfznkx", "kkbk", "tykczgxdcs",
}

// browserInfoParams 课程详情查询白名单 —— data.json classInfo(44)
var browserInfoParams = []string{
	"sfznkx", "zdkxms", "njdm_id_xs", "rlkz", "jxbzcxskg",
	"sfkkjyxdxnxq", "mzm", "rwlx", "kkbkdj", "xbm", "sfkkzy",
	"kzybkxy", "sfkcfx", "xkxnm", "njdm_id", "bh_id", "kkbk",
	"fxbj", "zyh_id", "gnjkxdnj", "zyh_id_xs", "ccdm", "sfkknj",
	"sfkxq", "xkxqm", "rlzlkz", "kch_id", "cxbj", "xqh_id",
	"jg_id", "xslbdm", "xz", "xsbj", "bbhzxjxb", "kklxdm",
	"xklc", "xkly", "bklx_id", "txbsfrl", "xkxskcgskg", "cdrlkz",
	"xkkz_id", "kzkcgs", "zyfx_id",
}

// selectParamNames 选课提交白名单（21 项，含 xkkz_xh）
//
// 构成：课程标识（kch_id/jxb_ids/kcmc/kklxdm）+ 会话参数
// （xkkz_id/xkkz_xh/njdm_id/zyh_id/xqh_id/jg_id）+ 学期（xkxnm/xkxqm）
// + 选课开关（rwlx/rlkz/rlzlkz/xklc/cdrlkz/xkly/sxbj/cxbj/qz）。
// 实机验证：用真实 do_jxb_id 提交，服务端能正确识别教学班与容量
// （flag=-1 时 msg 回显 `0,<jxb_id>,<容量>,`）；伪造 ID 则回"出现未知异常"。
var selectParamNames = []string{
	"kch_id", "jxb_ids", "kcmc", "kklxdm",
	"xkkz_id", "xkkz_xh", "njdm_id", "zyh_id", "xqh_id", "jg_id",
	"xkxnm", "xkxqm",
	"rwlx", "rlkz", "rlzlkz", "xklc", "cdrlkz", "xkly",
	"sxbj", "cxbj", "qz",
}

// cancelParamNames 退课白名单（只 5 个字段）
//
// 实机结论（2026-09-15，站点 JS zzxkYzbChoosedZy.js 的 delCourse()）：
// 退课端点 = /xsxk/zzxkyzb_tuikBcZzxkYzb.html（"tuik" 不是 "tk"！
// 原 tkZzxkYzb 服务端回「方法未定义」+ HTTP 910）。
var cancelParamNames = []string{
	"kch_id", "jxb_ids", "xkxnm", "xkxqm", "txbsfrl",
}

// displayParamNames Display（选课开关页）白名单 —— 浏览器同款 7 参数
// 其中 kspage/jspage 固定为 0；其响应下发的 hidden inputs
// （rwlx/xklc/rlkz/rlzlkz/cdrlkz/xkly 等）是第二阶段的参数来源。
var displayParamNames = []string{
	"xkkz_id", "kklxdm", "xszxzt", "njdm_id", "zyh_id",
	"kspage", "jspage",
}

// paramWhitelistSet 名单 → 集合（查询用）
var (
	browserListParamSet  = toSet(browserListParams)
	browserInfoParamSet  = toSet(browserInfoParams)
	selectParamSet       = toSet(selectParamNames)
	cancelParamSet       = toSet(cancelParamNames)
	displayParamSet      = toSet(displayParamNames)
)

// toSet 字符串切片 → 集合
func toSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}

// pickWhitelisted 从参数池中仅取出白名单内的字段。
//
// 关键点：**不在白名单内的字段一律丢弃**，即使页面下发过 ——
// 整包发送会被服务端以「系统运行异常」拒绝（见文件头对照实验）。
// 白名单内但池中缺失的字段 simply 不提交（缺参数会得到明确的服务端报错，
// 比"整包被拒但看不出原因"可排查得多）。
func pickWhitelisted(src map[string]string, set map[string]bool) map[string]string {
	out := make(map[string]string, len(set))
	for k, v := range src {
		if set[k] {
			out[k] = v
		}
	}
	return out
}
