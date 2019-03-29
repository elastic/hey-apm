package commands

type reports []TestReport

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

type descByThroughput template

func (wrap descByThroughput) Less(i, j int) bool {
	return wrap.reports[i].Throughput > wrap.reports[j].Throughput
}

type descByPushedVolume template

func (wrap descByPushedVolume) Less(i, j int) bool {
	return wrap.reports[i].PushedBps > wrap.reports[j].PushedBps
}

type descByDuration template

func (wrap descByDuration) Less(i, j int) bool {
	return wrap.reports[i].Duration.Seconds() > wrap.reports[j].Duration.Seconds()
}
