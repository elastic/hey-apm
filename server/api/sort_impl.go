package api

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

type descByRevDate template

func (wrap descByRevDate) Less(i, j int) bool {
	return wrap.reports[i].revisionDate().After(wrap.reports[j].revisionDate())
}

type ascByLatency template

func (wrap ascByLatency) Less(i, j int) bool {
	return wrap.reports[i].Latency < wrap.reports[j].Latency
}

type descByThroughput template

func (wrap descByThroughput) Less(i, j int) bool {
	return wrap.reports[i].Throughput > wrap.reports[j].Throughput
}

type descByEfficiency template

func (wrap descByEfficiency) Less(i, j int) bool {
	return wrap.reports[i].Efficiency > wrap.reports[j].Efficiency
}

type descByPushedVolume template

func (wrap descByPushedVolume) Less(i, j int) bool {
	return wrap.reports[i].PushedBps > wrap.reports[j].PushedBps
}

type descByDuration template

func (wrap descByDuration) Less(i, j int) bool {
	return wrap.reports[i].Duration.Seconds() > wrap.reports[j].Duration.Seconds()
}

type descByIndexSuccessRatio template

func (wrap descByIndexSuccessRatio) Less(i, j int) bool {
	return wrap.reports[i].ActualExpectRatio > wrap.reports[j].ActualExpectRatio
}
