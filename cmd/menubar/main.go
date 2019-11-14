package main

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/getlantern/systray"
	"github.com/golang/glog"
	"k8s.io/minikube/cmd/menubar/icon"
	"k8s.io/minikube/pkg/minikube/config"
	"k8s.io/minikube/pkg/minikube/kubeconfig"
)

type cluster struct {
	Name           string
	Config         *config.Config
	CurrentContext bool
	Running        bool
	Controllable   bool
	Type           string
}

/*

func healthIssues() {
	nodes, err := o.CoreClient.Nodes().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

}
*/

func activeClusters(ctx context.Context) ([]*cluster, error) {
	cs := []*cluster{}
	seen := map[string]*cluster{}

	mc, err := minikubeClusters(ctx)
	if err != nil {
		return cs, err
	}

	for _, c := range mc {
		glog.Infof("found minikube cluster: %+v", c)
		cs = append(cs, c)
		seen[c.Name] = c
	}

	cfg, err := kubeconfig.ReadOrNew()
	if err != nil {
		return cs, err
	}

	for k, v := range cfg.Clusters {
		if seen[k] == nil {
			// Stale minikube entry
			if strings.Contains(v.CertificateAuthority, "minikube") {
				continue
			}
			c := &cluster{Name: k}
			glog.Infof("found kubeconfig cluster: %+v", v)
			cs = append(cs, c)
			seen[k] = c
		}
	}
	if cfg.CurrentContext != "" && seen[cfg.CurrentContext] != nil {
		glog.Infof("setting current context to %s", cfg.CurrentContext)
		seen[cfg.CurrentContext].CurrentContext = true
	}
	return cs, nil
}

func minikubeClusters(ctx context.Context) ([]*cluster, error) {
	cmd := exec.CommandContext(ctx, "minikube", "profile", "list", "--output", "json")
	out, err := cmd.Output()
	glog.Infof("err: %v output: %s\n", err, out)
	if err != nil {
		return nil, err
	}
	var ps map[string][]config.Profile
	err = json.Unmarshal(out, &ps)
	if err != nil {
		return nil, err
	}

	cs := []*cluster{}
	for _, p := range ps["valid"] {
		glog.Infof("valid minikube cluster: %+v", p)
		c := &cluster{
			Name:         p.Name,
			Config:       p.Config,
			Controllable: true,
			Type:         "minikube",
		}
		if p.Config.KubernetesConfig.NodeIP != "" {
			c.Running = true
		}
		cs = append(cs, c)
	}
	return cs, nil
}

func main() {
	onExit := func() {
		glog.Infof("exiting")
	}
	// Should be called at the very beginning of main().
	systray.Run(onReady, onExit)
}

func addClusterActions(c *cluster, i *systray.MenuItem) {
	start := i.AddSubMenuItem("Start", "Start the cluster")
	go func() {
		<-start.ClickedCh
		cmd := exec.CommandContext(context.Background(), "minikube", "start", "-p", c.Name, "--wait=false")
		err := cmd.Run()
		if err != nil {
			glog.Errorf("start failed: %v", err)
		}
	}()

	stop := i.AddSubMenuItem("Stop", "Stop the cluster")
	go func() {
		<-stop.ClickedCh
		cmd := exec.CommandContext(context.Background(), "minikube", "stop", "-p", c.Name)
		err := cmd.Run()
		if err != nil {
			glog.Errorf("stop failed: %v", err)
		}
	}()

	delete := i.AddSubMenuItem("Delete", "Delete the cluster")
	go func() {
		<-delete.ClickedCh
		cmd := exec.CommandContext(context.Background(), "minikube", "delete", "-p", c.Name)
		err := cmd.Run()
		if err != nil {
			glog.Errorf("delete failed: %v", err)
		}
	}()

	dashboard := i.AddSubMenuItem("Dashboard", "Display Dashboard")
	go func() {
		<-dashboard.ClickedCh
		cmd := exec.CommandContext(context.Background(), "minikube", "dashboard", "-p", c.Name)
		err := cmd.Run()
		if err != nil {
			glog.Errorf("stop failed: %v", err)
		}
	}()

	tunnel := i.AddSubMenuItem("Tunnel", "Start Tunnel")
	go func() {
		<-tunnel.ClickedCh
		cmd := exec.CommandContext(context.Background(), "minikube", "tunnel", "-p", c.Name)
		err := cmd.Run()
		if err != nil {
			glog.Errorf("tunnel failed: %v", err)
		}
	}()
}

func updateMenu(ci map[string]*systray.MenuItem) {
	ctx := context.Background()
	cs, err := activeClusters(ctx)
	if err != nil {
		glog.Errorf("error retrieving clusters: %v", err)
	}

	for _, c := range cs {
		// This cluster is already being tracked.
		if ci[c.Name] != nil {
			if c.CurrentContext {
				systray.SetTitle(c.Name)
			}
			continue
		}

		glog.Infof("Adding menu item for: %+v", c)
		ci[c.Name] = systray.AddMenuItem(c.Name, "")
		if c.Controllable {
			addClusterActions(c, ci[c.Name])
		}
		if c.Type == "minikube" {
			ci[c.Name].SetIcon(icon.Data)
		}
		if c.CurrentContext {
			ci[c.Name].Check()
			systray.SetTitle(c.Name)
		}
	}
}

/*
	mChange := systray.AddMenuItem("Start", "Start")
	mChecked := systray.AddMenuItem("Unchecked", "Check Me")
	mEnabled := systray.AddMenuItem("Enabled", "Enabled")
	systray.AddMenuItem("Ignored", "Ignored")
	mUrl := systray.AddMenuItem("Open Lantern.org", "my home")
	mQuit := systray.AddMenuItem("退出", "Quit the whole app")

	// Sets the icon of a menu item. Only available on Mac.
	mQuit.SetIcon(icon.Data)

	systray.AddSeparator()
	mToggle := systray.AddMenuItem("Toggle", "Toggle the Quit button")
	shown := true
	for {
		select {
		case <-mChange.ClickedCh:
			mChange.SetTitle("I've Changed")
		case <-mChecked.ClickedCh:
			if mChecked.Checked() {
				mChecked.Uncheck()
				mChecked.SetTitle("Unchecked")
			} else {
				mChecked.Check()
				mChecked.SetTitle("Checked")
			}
		case <-mEnabled.ClickedCh:
			mEnabled.SetTitle("Disabled")
			mEnabled.Disable()
		case <-mUrl.ClickedCh:
			open.Run("https://www.getlantern.org")
		case <-mToggle.ClickedCh:
			if shown {
				mQuitOrig.Hide()
				mEnabled.Hide()
				shown = false
			} else {
				mQuitOrig.Show()
				mEnabled.Show()
				shown = true
			}
		case <-mQuit.ClickedCh:
			systray.Quit()
			fmt.Println("Quit2 now...")
			return
		}
	}

*/

func onReady() {
	systray.SetIcon(icon.Data)
	systray.SetTooltip("Local Kubernetes")
	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	clusterItems := map[string]*systray.MenuItem{}
	go func() {

		for {
			updateMenu(clusterItems)
			time.Sleep(2 * time.Second)
		}
	}()
}
