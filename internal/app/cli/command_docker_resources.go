package cli

import (
	"errors"
	"fmt"
	"strings"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

const dockerResourceCommandGet = "get"

func (application *app) runNetwork(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: composia network <node> <list|get|remove>")
	}
	nodeID := strings.TrimSpace(args[0])
	if nodeID == "" {
		return errors.New("node is required")
	}
	switch args[1] {
	case "list":
		return application.runNetworkList(nodeID, args[2:])
	case dockerResourceCommandGet:
		return application.runNetworkGet(nodeID, args[2:])
	case "remove":
		return application.runNetworkRemove(nodeID, args[2:])
	default:
		return fmt.Errorf("unknown network command %q", args[1])
	}
}

func (application *app) runNetworkList(nodeID string, args []string) error {
	search, sortBy, sortDesc, pageValues, err := parseDockerResourceListFlags("network list", args, "composia network <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]")
	if err != nil {
		return err
	}
	pageSize, page, err := pageValues()
	if err != nil {
		return err
	}
	response, err := application.client.docker.ListNodeNetworks(application.ctx, newRequest(&controllerv1.ListNodeNetworksRequest{NodeId: nodeID, PageSize: pageSize, Page: page, Search: search, SortBy: sortBy, SortDesc: sortDesc}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetNetworks()))
	for _, network := range response.Msg.GetNetworks() {
		rows = append(rows, []string{network.GetId(), network.GetName(), network.GetDriver(), network.GetScope(), boolText(network.GetInternal()), boolText(network.GetAttachable()), uintText(network.GetContainersCount()), network.GetSubnet(), network.GetGateway()})
	}
	if err := application.writeTable([]string{"NETWORK", "NAME", "DRIVER", "SCOPE", "INTERNAL", "ATTACHABLE", "CONTAINERS", "SUBNET", "GATEWAY"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runNetworkGet(nodeID string, args []string) error {
	id, err := parseDockerResourceGetFlags("network get", args, "composia network <node> get <network>")
	if err != nil {
		return err
	}
	response, err := application.client.docker.InspectNodeNetwork(application.ctx, newRequest(&controllerv1.InspectNodeNetworkRequest{NodeId: nodeID, NetworkId: id}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprintln(application.out, response.Msg.GetRawJson())
	return err
}

func (application *app) runNetworkRemove(nodeID string, args []string) error {
	id, waitOptions, err := parseDockerResourceRemoveFlags("network remove", args, "composia network <node> remove [--wait] [--follow] [--timeout duration] <network>")
	if err != nil {
		return err
	}
	response, err := application.client.dockerCommands.RemoveNetwork(application.ctx, newRequest(&controllerv1.RemoveNetworkRequest{NodeId: nodeID, NetworkId: id}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runVolume(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: composia volume <node> <list|get|remove>")
	}
	nodeID := strings.TrimSpace(args[0])
	if nodeID == "" {
		return errors.New("node is required")
	}
	switch args[1] {
	case "list":
		return application.runVolumeList(nodeID, args[2:])
	case dockerResourceCommandGet:
		return application.runVolumeGet(nodeID, args[2:])
	case "remove":
		return application.runVolumeRemove(nodeID, args[2:])
	default:
		return fmt.Errorf("unknown volume command %q", args[1])
	}
}

func (application *app) runVolumeList(nodeID string, args []string) error {
	search, sortBy, sortDesc, pageValues, err := parseDockerResourceListFlags("volume list", args, "composia volume <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]")
	if err != nil {
		return err
	}
	pageSize, page, err := pageValues()
	if err != nil {
		return err
	}
	response, err := application.client.docker.ListNodeVolumes(application.ctx, newRequest(&controllerv1.ListNodeVolumesRequest{NodeId: nodeID, PageSize: pageSize, Page: page, Search: search, SortBy: sortBy, SortDesc: sortDesc}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetVolumes()))
	for _, volume := range response.Msg.GetVolumes() {
		rows = append(rows, []string{volume.GetName(), volume.GetDriver(), volume.GetScope(), int64Text(volume.GetSizeBytes()), uintText(volume.GetContainersCount()), boolText(volume.GetInUse()), volume.GetMountpoint()})
	}
	if err := application.writeTable([]string{"VOLUME", "DRIVER", "SCOPE", "SIZE_BYTES", "CONTAINERS", "IN_USE", "MOUNTPOINT"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runVolumeGet(nodeID string, args []string) error {
	id, err := parseDockerResourceGetFlags("volume get", args, "composia volume <node> get <volume>")
	if err != nil {
		return err
	}
	response, err := application.client.docker.InspectNodeVolume(application.ctx, newRequest(&controllerv1.InspectNodeVolumeRequest{NodeId: nodeID, VolumeName: id}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprintln(application.out, response.Msg.GetRawJson())
	return err
}

func (application *app) runVolumeRemove(nodeID string, args []string) error {
	id, waitOptions, err := parseDockerResourceRemoveFlags("volume remove", args, "composia volume <node> remove [--wait] [--follow] [--timeout duration] <volume>")
	if err != nil {
		return err
	}
	response, err := application.client.dockerCommands.RemoveVolume(application.ctx, newRequest(&controllerv1.RemoveVolumeRequest{NodeId: nodeID, VolumeName: id}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runImage(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: composia image <node> <list|get|remove>")
	}
	nodeID := strings.TrimSpace(args[0])
	if nodeID == "" {
		return errors.New("node is required")
	}
	switch args[1] {
	case "list":
		return application.runImageList(nodeID, args[2:])
	case dockerResourceCommandGet:
		return application.runImageGet(nodeID, args[2:])
	case "remove":
		return application.runImageRemove(nodeID, args[2:])
	default:
		return fmt.Errorf("unknown image command %q", args[1])
	}
}

func (application *app) runImageList(nodeID string, args []string) error {
	search, sortBy, sortDesc, pageValues, err := parseDockerResourceListFlags("image list", args, "composia image <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]")
	if err != nil {
		return err
	}
	pageSize, page, err := pageValues()
	if err != nil {
		return err
	}
	response, err := application.client.docker.ListNodeImages(application.ctx, newRequest(&controllerv1.ListNodeImagesRequest{NodeId: nodeID, PageSize: pageSize, Page: page, Search: search, SortBy: sortBy, SortDesc: sortDesc}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetImages()))
	for _, image := range response.Msg.GetImages() {
		rows = append(rows, []string{image.GetId(), strings.Join(image.GetRepoTags(), ","), int64Text(image.GetSize()), image.GetCreated(), image.GetArchitecture(), image.GetOs(), uintText(image.GetContainersCount()), boolText(image.GetIsDangling())})
	}
	if err := application.writeTable([]string{"IMAGE", "TAGS", "SIZE_BYTES", "CREATED", "ARCH", "OS", "CONTAINERS", "DANGLING"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runImageGet(nodeID string, args []string) error {
	id, err := parseDockerResourceGetFlags("image get", args, "composia image <node> get <image>")
	if err != nil {
		return err
	}
	response, err := application.client.docker.InspectNodeImage(application.ctx, newRequest(&controllerv1.InspectNodeImageRequest{NodeId: nodeID, ImageId: id}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprintln(application.out, response.Msg.GetRawJson())
	return err
}

func (application *app) runImageRemove(nodeID string, args []string) error {
	fs := newCommandFlagSet("image remove")
	force := fs.Bool("force", false, "force remove")
	waitOptions := addWaitFlags(fs)
	usage := "composia image <node> remove [--wait] [--follow] [--timeout duration] [--force] <image>"
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	response, err := application.client.dockerCommands.RemoveImage(application.ctx, newRequest(&controllerv1.RemoveImageRequest{NodeId: nodeID, ImageId: fs.Arg(0), Force: *force}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func parseDockerResourceListFlags(name string, args []string, usage string) (string, string, bool, func() (uint32, uint32, error), error) {
	fs := newCommandFlagSet(name)
	search := fs.String("search", "", "search text")
	sortBy := fs.String("sort-by", "", "sort field")
	sortDesc := fs.Bool("desc", false, "sort descending")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return "", "", false, nil, err
	}
	if err := requireArgs(fs.Args(), 0, usage); err != nil {
		return "", "", false, nil, err
	}
	return *search, *sortBy, *sortDesc, pageValues, nil
}

func parseDockerResourceGetFlags(name string, args []string, usage string) (string, error) {
	fs := newCommandFlagSet(name)
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return "", err
	}
	return fs.Arg(0), nil
}

func parseDockerResourceRemoveFlags(name string, args []string, usage string) (string, waitOptions, error) {
	fs := newCommandFlagSet(name)
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return "", waitOptions, err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return "", waitOptions, err
	}
	return fs.Arg(0), waitOptions, nil
}
