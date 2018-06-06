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
	return wrap.reports[i].latency() < wrap.reports[j].latency()
}

type descByThroughput template

func (wrap descByThroughput) Less(i, j int) bool {
	return wrap.reports[i].throughput() > wrap.reports[j].throughput()
}

type descByEfficiency template

func (wrap descByEfficiency) Less(i, j int) bool {
	return wrap.reports[i].efficiency() > wrap.reports[j].efficiency()
}

type descByPushedVolume template

func (wrap descByPushedVolume) Less(i, j int) bool {
	return wrap.reports[i].pushedVolumePerSecond() > wrap.reports[j].pushedVolumePerSecond()
}

type descByDuration template

func (wrap descByDuration) Less(i, j int) bool {
	return wrap.reports[i].Duration.Seconds() > wrap.reports[j].Duration.Seconds()
}

type descByIndexSuccessRatio template

func (wrap descByIndexSuccessRatio) Less(i, j int) bool {
	return wrap.reports[i].indexSuccessRatio() > wrap.reports[j].indexSuccessRatio()
}
