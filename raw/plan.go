package raw

import pathplan "github.com/icloudza/gcjson/internal"

type PathPlan = pathplan.Plan

func CompilePath(p string) *PathPlan       { return pathplan.Compile(p) }
func CompilePaths(ps []string) []*PathPlan { return pathplan.CompileMany(ps) }
func GetByPlan(doc []byte, pl *PathPlan) (string, bool) {
	bs, ok := GetBytesByPlan(doc, pl)
	return string(bs), ok
}
func GetManyByPlan(doc []byte, pls []*PathPlan) ([]string, []bool) {
	bss, oks := GetManyBytesByPlan(doc, pls)
	out := make([]string, len(bss))
	for i := range bss {
		if oks[i] {
			out[i] = string(bss[i])
		}
	}
	return out, oks
}
