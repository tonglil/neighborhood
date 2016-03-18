package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	gostats "github.com/GaryBoone/GoStats/stats"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func main() {
	topPtr := flag.Int("top", 0, "The number of nodes to count an average rtt for [0 = all]")
	flag.Parse()

	fmt.Println("Determining what the neighborhood looks like!")
	fmt.Println("Finding the rtts from Consul...\n")

	cmd := exec.Command("consul", "members")
	printCommand(cmd)

	stdout, err := cmd.StdoutPipe()
	check(err)
	err = cmd.Start()
	check(err)
	scanner := bufio.NewScanner(stdout)

	nodes := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()

		// Do not add nodes that are commented out
		if len(line) > 0 && line[0:4] != "Node" {
			r, _ := regexp.Compile("(.+?) ")
			x := r.FindString(line)
			x = strings.Trim(x, " \n")

			if len(x) > 0 {
				nodes[x] = x
			}
		}
	}

	err = scanner.Err()
	check(err)

	err = cmd.Wait()
	check(err)

	ranked := rtt(nodes)

	for rank, pair := range ranked {
		fmt.Printf("%v: %v - %vms\n", rank, pair.Key, pair.Value)
	}

	data := stats(ranked, *topPtr)

	fmt.Printf("\nFor the top %v nodes...\n", data.Count())
	fmt.Printf("Average: %vms, Min: %vms, Max: %vms\n", data.Mean(), data.Min(), data.Max())
	fmt.Printf("Standard deviation: %v, Variance: %v\n", data.PopulationStandardDeviation(), data.PopulationVariance())

	return
}

func stats(nodes PairList, count int) gostats.Stats {
	if count == 0 {
		count = len(nodes)
	}

	var d gostats.Stats

	for _, pair := range nodes[:count] {
		d.Update(pair.Value)
	}

	return d
}

func rtt(nodes map[string]string) PairList {
	times := make(map[string]float64)

	r, _ := regexp.Compile("\\d+.\\d+")

	for _, node := range nodes {
		cmd := exec.Command("consul", "rtt", node)
		output, err := cmd.CombinedOutput()
		if err != nil {
			printCommand(cmd)
			printError(err)
			printOutput(output)
			continue
		}

		s := string(output)
		time := r.FindString(s)
		f, err := strconv.ParseFloat(time, 64)
		if err != nil {
			continue
		}

		times[node] = f
	}

	return ranker(times)
}

func printCommand(cmd *exec.Cmd) {
	fmt.Printf("==> Executing: %s\n", strings.Join(cmd.Args, " "))
}

func printError(err error) {
	if err != nil {
		os.Stderr.WriteString(fmt.Sprintf("==> Error: %s\n", err.Error()))
	}
}

func printOutput(outs []byte) {
	if len(outs) > 0 {
		fmt.Printf("==> Output: %s\n", string(outs))
	}
}

func ranker(nodes map[string]float64) PairList {
	pl := make(PairList, len(nodes))
	i := 0

	for k, v := range nodes {
		pl[i] = Pair{k, v}
		i++
	}

	sort.Sort(pl)

	return pl
}

type Pair struct {
	Key   string
	Value float64
}

type PairList []Pair

func (p PairList) Len() int           { return len(p) }
func (p PairList) Less(i, j int) bool { return p[i].Value < p[j].Value }
func (p PairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
