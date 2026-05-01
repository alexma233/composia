package cli

import (
	"fmt"
	"strings"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runNetwork(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia network <list|get|remove>")
	}
	switch args[0] {
	case "list":
		return application.runNetworkList(args[1:])
	case "get":
		return application.runNetworkGet(args[1:])
	case "remove":
		return application.runNetworkRemove(args[1:])
	default:
		return fmt.Errorf("unknown network command %q", args[0])
	}
}

func (application *app) runNetworkList(args []string) error {
	nodeID, search, sortBy, sortDesc, pageValues, err := parseDockerResourceListFlags("network list", args, "composia network list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]")
	if err != nil {
		return err
	}
	pageSize, page := pageValues()
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

func (application *app) runNetworkGet(args []string) error {
	nodeID, id, err := parseDockerResourceGetFlags("network get", args, "composia network get --node node <network>")
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

func (application *app) runNetworkRemove(args []string) error {
	nodeID, id, waitOptions, err := parseDockerResourceRemoveFlags("network remove", args, "composia network remove [--wait] [--follow] [--timeout duration] --node node <network>")
	if err != nil {
		return err
	}
	response, err := application.client.containers.RemoveNetwork(application.ctx, newRequest(&controllerv1.RemoveNetworkRequest{NodeId: nodeID, NetworkId: id}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runVolume(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia volume <list|get|remove>")
	}
	switch args[0] {
	case "list":
		return application.runVolumeList(args[1:])
	case "get":
		return application.runVolumeGet(args[1:])
	case "remove":
		return application.runVolumeRemove(args[1:])
	default:
		return fmt.Errorf("unknown volume command %q", args[0])
	}
}

func (application *app) runVolumeList(args []string) error {
	nodeID, search, sortBy, sortDesc, pageValues, err := parseDockerResourceListFlags("volume list", args, "composia volume list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]")
	if err != nil {
		return err
	}
	pageSize, page := pageValues()
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

func (application *app) runVolumeGet(args []string) error {
	nodeID, id, err := parseDockerResourceGetFlags("volume get", args, "composia volume get --node node <volume>")
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

func (application *app) runVolumeRemove(args []string) error {
	nodeID, id, waitOptions, err := parseDockerResourceRemoveFlags("volume remove", args, "composia volume remove [--wait] [--follow] [--timeout duration] --node node <volume>")
	if err != nil {
		return err
	}
	response, err := application.client.containers.RemoveVolume(application.ctx, newRequest(&controllerv1.RemoveVolumeRequest{NodeId: nodeID, VolumeName: id}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runImage(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia image <list|get|remove>")
	}
	switch args[0] {
	case "list":
		return application.runImageList(args[1:])
	case "get":
		return application.runImageGet(args[1:])
	case "remove":
		return application.runImageRemove(args[1:])
	default:
		return fmt.Errorf("unknown image command %q", args[0])
	}
}

func (application *app) runImageList(args []string) error {
	nodeID, search, sortBy, sortDesc, pageValues, err := parseDockerResourceListFlags("image list", args, "composia image list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]")
	if err != nil {
		return err
	}
	pageSize, page := pageValues()
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

func (application *app) runImageGet(args []string) error {
	nodeID, id, err := parseDockerResourceGetFlags("image get", args, "composia image get --node node <image>")
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

func (application *app) runImageRemove(args []string) error {
	fs := newCommandFlagSet("image remove")
	nodeID := fs.String("node", "", "node ID")
	force := fs.Bool("force", false, "force remove")
	waitOptions := addWaitFlags(fs)
	usage := "composia image remove [--wait] [--follow] [--timeout duration] --node node [--force] <image>"
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", usage)
	}
	response, err := application.client.containers.RemoveImage(application.ctx, newRequest(&controllerv1.RemoveImageRequest{NodeId: strings.TrimSpace(*nodeID), ImageId: fs.Arg(0), Force: *force}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func parseDockerResourceListFlags(name string, args []string, usage string) (string, string, string, bool, func() (uint32, uint32), error) {
	fs := newCommandFlagSet(name)
	nodeID := fs.String("node", "", "node ID")
	search := fs.String("search", "", "search text")
	sortBy := fs.String("sort-by", "", "sort field")
	sortDesc := fs.Bool("desc", false, "sort descending")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return "", "", "", false, nil, err
	}
	if err := requireArgs(fs.Args(), 0, usage); err != nil {
		return "", "", "", false, nil, err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return "", "", "", false, nil, errorsWithUsage("node is required", usage)
	}
	return strings.TrimSpace(*nodeID), *search, *sortBy, *sortDesc, pageValues, nil
}

func parseDockerResourceGetFlags(name string, args []string, usage string) (string, string, error) {
	fs := newCommandFlagSet(name)
	nodeID := fs.String("node", "", "node ID")
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return "", "", err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return "", "", errorsWithUsage("node is required", usage)
	}
	return strings.TrimSpace(*nodeID), fs.Arg(0), nil
}

func parseDockerResourceRemoveFlags(name string, args []string, usage string) (string, string, waitOptions, error) {
	fs := newCommandFlagSet(name)
	nodeID := fs.String("node", "", "node ID")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return "", "", waitOptions, err
	}
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return "", "", waitOptions, err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return "", "", waitOptions, errorsWithUsage("node is required", usage)
	}
	return strings.TrimSpace(*nodeID), fs.Arg(0), waitOptions, nil
}
