package console

import (
	"github.com/tucats/govax/internal/dcl"
	"github.com/tucats/govax/internal/dclcmd"
)

// bindDifferences binds DIFFERENCES (docs/planning/PHASE-116 -
// DIFFERENCES.md; internal/dclcmd/differences.go) to c: each qualifier as it
// was given, the command reading the values itself.
func bindDifferences(g *dcl.Grammar, c *Console) {
	g.Bind("DIFFERENCES", func(id int64, r *dcl.Result) error {
		value := func(name string) dclcmd.DifferencesValue {
			return dclcmd.DifferencesValue{
				Given: r.Present(name) && !r.Defaulted(name), Negated: r.Negated(name), Text: r.String(name),
			}
		}

		bar := func(param string) (dclcmd.DifferencesValue, []string) {
			v := dclcmd.DifferencesValue{
				Given:   r.ParamPresent(param, "CHANGE_BAR") && !r.ParamDefaulted(param, "CHANGE_BAR"),
				Negated: r.ParamNegated(param, "CHANGE_BAR"),
			}

			return v, r.ParamList(param, "CHANGE_BAR")
		}

		req := dclcmd.DifferencesRequest{
			Master: r.String("MASTER"), Revision: r.String("REVISION"),
			Ignore: value("IGNORE"), IgnoreList: r.List("IGNORE"),
			CommentDelimiter: value("COMMENT_DELIMITER"), CommentList: r.List("COMMENT_DELIMITER"),
			Mode: value("MODE"), ModeList: r.List("MODE"),
			Separated: value("SEPARATED"), SeparatedList: r.List("SEPARATED"),
			Match: value("MATCH"), Merged: value("MERGED"), MaximumDifferences: value("MAXIMUM_DIFFERENCES"),
			Parallel: value("PARALLEL"), Width: value("WIDTH"), Window: value("WINDOW"),
			Number: value("NUMBER"), Output: value("OUTPUT"), SLP: value("SLP"),
		}

		req.MasterBar, req.MasterBarList = bar("MASTER")
		req.RevisionBar, req.RevisionBarList = bar("REVISION")

		return c.cmd().Differences(req)
	})
}
