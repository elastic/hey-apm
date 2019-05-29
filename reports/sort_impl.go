package reports

type reports []Report

func (r reports) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r reports) Len() int {
	return len(r)
}

type template struct {
	reports
}

type descByReportDate template

func (wrap descByReportDate) Less(i, j int) bool {
	return wrap.reports[i].date().After(wrap.reports[j].date())
}

type descByDuration template

func (wrap descByDuration) Less(i, j int) bool {
	return wrap.reports[i].RunTimeout > wrap.reports[j].RunTimeout
}
