package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tap-dev/tap/internal/discovery"
	"github.com/tap-dev/tap/internal/model"
)

type portRow struct {
	Port     uint16 `json:"port"`
	PID      *int32 `json:"pid,omitempty"`
	Name     string `json:"name"`
	Project  string `json:"project"`
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
}

var portsCmd = &cobra.Command{
	Use:   "ports",
	Short: "List all ports in use by dev processes",
	Long:  "Displays every port currently in use across all discovered dev processes, sorted by port number.",
	RunE:  portsRun,
}

func init() {
	rootCmd.AddCommand(portsCmd)
}

func portsRun(cmd *cobra.Command, args []string) error {
	snap, err := discovery.TakeSnapshot(!noDocker)
	if err != nil {
		return fmt.Errorf("snapshot failed: %w", err)
	}

	rows := collectPortRows(snap)

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Port < rows[j].Port
	})

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	if len(rows) == 0 {
		fmt.Println("No ports in use.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "PORT\tPID\tNAME\tPROJECT\tPROTOCOL\tADDRESS")

	for _, r := range rows {
		pidStr := "-"
		if r.PID != nil {
			pidStr = strconv.Itoa(int(*r.PID))
		}

		project := r.Project
		if project == "" {
			project = "-"
		}

		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
			r.Port,
			pidStr,
			r.Name,
			project,
			r.Protocol,
			r.Address,
		)
	}

	return w.Flush()
}

func collectPortRows(snap *model.Snapshot) []portRow {
	var rows []portRow

	addFromProcesses := func(procs []model.DevProcess) {
		for i := range procs {
			p := &procs[i]
			for _, pb := range p.Ports {
				rows = append(rows, portRow{
					Port:     pb.Port,
					PID:      p.PID,
					Name:     p.Name,
					Project:  p.Project,
					Protocol: pb.Protocol,
					Address:  pb.Address,
				})
			}
		}
	}

	for i := range snap.Projects {
		addFromProcesses(snap.Projects[i].Processes)
	}
	addFromProcesses(snap.Unattributed)

	return rows
}
