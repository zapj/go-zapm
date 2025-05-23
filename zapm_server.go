package zapm

import (
	"fmt"
	"net/http"
)

func showServices(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "%-10s | %-20s | %-20s | %-10s | %6s\n", "Name", "Title", "Run", "StartupType", "Status")
	for _, s := range Conf.Services {
		fmt.Fprintf(w, "%-10s | %-20s | %-20s | %-10s | %6s\n", s.Name, s.Title, s.Run, s.StartupType, s.Status)
	}
}

func StartMonitorServer() error {
	http.HandleFunc("/show_services", showServices)
	err := http.ListenAndServe(fmt.Sprintf("%s:%d", Conf.Server.Address, Conf.Server.Port), nil)
	if err != nil {
		return err
	}
	return nil
}
