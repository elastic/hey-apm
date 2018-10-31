package output

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/elastic/hey/requester"
)

func PrintResults(w *requester.Work, dur float64, writer io.Writer) {
	statusCodeDist := w.StatusCodes()
	codes, total := SortedTotal(statusCodeDist)
	div := float64(total)
	for _, code := range codes {
		cnt := statusCodeDist[code]
		fmt.Fprintf(writer, "[%d] %d responses (%.2f%%) \n", code, cnt, 100*float64(cnt)/div)
	}
	fmt.Fprintf(writer, "total %d responses (%.2f rps)\n", total, div/dur)

	errorTotal := 0
	errorDist := make(map[string]int)
	for err, num := range w.ErrorDist() {
		err = collapseError(err)
		errorDist[err] += num
		errorTotal += num
	}

	if errorTotal > 0 {
		errorKeys := sortedErrors(errorDist)
		fmt.Fprintf(writer, "\n  %d errors:\n", errorTotal)
		for _, err := range errorKeys {
			num := errorDist[err]
			fmt.Fprintf(writer, "  [%d]\t%s\n", num, err)
		}
	}
}

// sortedTotalErrors sorts by the values of the input
func sortedErrors(m map[string]int) []string {
	errorCounts := make(errorCountSlice, len(m))
	i := 0
	for k, v := range m {
		errorCounts[i] = errorCount{err: k, value: v}
		i++
	}
	sort.Sort(sort.Reverse(errorCounts))
	keys := make([]string, len(errorCounts))
	for i, e := range errorCounts {
		keys[i] = e.err
	}
	return keys
}

// collapseError makes groups of similar errors identical
func collapseError(e string) string {
	// Post http://localhost:8201/v1/transactions: read tcp 127.0.0.1:63204->127.0.0.1:8201: read: connection reset by peer
	if strings.HasSuffix(e, "read: connection reset by peer") {
		return "read: connection reset by peer"
	}

	// Post http://localhost:8200/v1/transactions: net/http: HTTP/1.x transport connection broken: write tcp [::1]:63967->[::1]:8200: write: broken pipe
	if strings.HasSuffix(e, "write: broken pipe") {
		return "write: broken pipe"
	}
	return e
}

// SortedTotal sorts the keys and sums the values of the input map
func SortedTotal(m map[int]int) ([]int, int) {
	keys := make([]int, len(m))
	i := 0
	total := 0
	for k, v := range m {
		keys[i] = k
		total += v
		i++
	}
	sort.Ints(keys)
	return keys, total
}

// errorCount is for sorting errors by count
type errorCount struct {
	err   string
	value int
}
type errorCountSlice []errorCount

func (e errorCountSlice) Len() int           { return len(e) }
func (e errorCountSlice) Less(i, j int) bool { return e[i].value < e[j].value }
func (e errorCountSlice) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }
