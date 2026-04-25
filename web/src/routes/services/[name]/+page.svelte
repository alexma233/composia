<script lang="ts">
  import CheckIcon from "@lucide/svelte/icons/check";
  import ChevronsUpDownIcon from "@lucide/svelte/icons/chevrons-up-down";
  import { goto, invalidateAll } from "$app/navigation";
  import { onMount } from "svelte";
  import {
    Columns2,
    Copy,
    FilePlus,
    FolderPlus,
    Lock,
    Pencil,
    Play,
    RefreshCcw,
    Save,
    Square,
    Trash2,
    Upload,
    Wrench,
  } from "lucide-svelte";

  import type { PageData } from "./$types";
  import {
    actionErrorMessage,
    capabilityReasonMessage,
    serviceActionCapability,
  } from "$lib/capabilities";
  import DisabledReasonTooltip from "$lib/components/app/disabled-reason-tooltip.svelte";
  import { messages } from "$lib/i18n";

  import CodeEditor from "$lib/components/app/code-editor.svelte";
  import ServiceFileTree from "$lib/components/app/service-file-tree.svelte";
  import TaskItem from "$lib/components/app/task-item.svelte";
  import {
    Alert,
    AlertDescription,
    AlertTitle,
  } from "$lib/components/ui/alert";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import {
    Collapsible,
    CollapsibleContent,
    CollapsibleTrigger,
  } from "$lib/components/ui/collapsible";
  import {
    Dialog,
    DialogTitle,
    DialogDescription,
    DialogFooter,
    DialogHeader,
  } from "$lib/components/ui/dialog";
  import * as Command from "$lib/components/ui/command";
  import DialogContent from "$lib/components/ui/dialog/dialog-content.svelte";
  import DialogOverlay from "$lib/components/ui/dialog/dialog-overlay.svelte";
  import { Input } from "$lib/components/ui/input";
  import * as Popover from "$lib/components/ui/popover";
  import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
  } from "$lib/components/ui/select";
  import { toast } from "svelte-sonner";
  import {
    formatTimestamp,
    runtimeStatusLabel,
    runtimeStatusTone,
    taskStatusLabel,
    taskStatusTone,
  } from "$lib/presenters";
  import type {
    BackupSummary,
    RepoWriteResult,
    ServiceActionResult,
    ServiceInstanceDetail,
    TaskSummary,
  } from "$lib/server/controller";
  import type { ServiceFileNode, WorkspaceFile } from "$lib/service-workspace";
  import {
    findNode,
    isEncryptedFilePath,
    normalizeServiceRelativePath,
  } from "$lib/service-workspace";
  import { startPolling } from "$lib/refresh";
  import { cn } from "$lib/utils";

  let { data }: { data: PageData } = $props();

  type EditorTab = WorkspaceFile & {
    savedContent: string;
    dirty: boolean;
  };

  let fileTree = $state<ServiceFileNode[]>([]);
  let collapsedPaths = $state(new Set<string>());
  let selectedNodePath = $state("");
  let openTabs = $state<EditorTab[]>([]);
  let activePath = $state("");
  let secondaryPath = $state("");
  let splitEnabled = $state(false);
  let focusedPane = $state<"primary" | "secondary">("primary");
  let headRevision = $state("");
  let syncStatus = $state("");
  let syncError = $state("");
  let lastSuccessfulPullAt = $state("");
  let tasks = $state<TaskSummary[]>([]);
  let backups = $state<BackupSummary[]>([]);
  let actionBusy = $state("");
  let saving = $state(false);
  let errorMessage = $state("");
  let showNewFile = $state(false);
  let newFilePath = $state("");
  let showNewFolder = $state(false);
  let newFolderPath = $state("");
  let showRename = $state(false);
  let renamePath = $state("");
  let showDeleteDialog = $state(false);
  let showServiceRename = $state(false);
  let renameServiceFolder = $state("");
  let advancedOperationsOpen = $state(false);
  let workspace = $state<PageData["workspace"]>(null);
  let serviceDetail = $state<PageData["serviceDetail"]>(null);
  let nodeContainers = $state<NonNullable<PageData["nodeContainers"]>>([]);
  let instanceLoadState = $state<
    Record<string, "idle" | "loading" | "loaded" | "error">
  >({});
  let instanceLoadError = $state<Record<string, string>>({});
  let stopActionRefreshHandle = $state<null | (() => void)>(null);
  let migrateSourceNode = $state("");
  let migrateTargetNode = $state("");
  let selectedInstanceNode = $state("__all__");
  let serviceSwitchOpen = $state(false);
  let fileTreeIconTheme = $state<"light" | "dark">("dark");

  type ServiceSummaryStatePayload = {
    workspace?: PageData["workspace"];
    tasks?: TaskSummary[];
    backups?: BackupSummary[];
    serviceDetail?: PageData["serviceDetail"];
  };

  $effect(() => {
    fileTree = data.fileTree;
    openTabs = data.initialFile ? [createTab(data.initialFile)] : [];
    selectedNodePath = data.initialFile?.path ?? "";
    activePath = data.initialFile?.path ?? "";
    secondaryPath = "";
    splitEnabled = false;
    focusedPane = "primary";
    headRevision = data.repoHead?.headRevision ?? "";
    syncStatus = data.repoHead?.syncStatus ?? "";
    syncError = data.repoHead?.lastSyncError ?? "";
    lastSuccessfulPullAt = data.repoHead?.lastSuccessfulPullAt ?? "";
    errorMessage = data.error ?? "";
    renameServiceFolder = data.workspace?.folder ?? "";
    resetServiceSummaryState({
      workspace: data.workspace,
      tasks: data.tasks,
      backups: data.backups,
      serviceDetail: data.serviceDetail,
    });
    migrateSourceNode = data.serviceDetail?.nodes?.[0] ?? "";
    migrateTargetNode = "";
    selectedInstanceNode = "__all__";
  });

  function resetServiceSummaryState(payload: ServiceSummaryStatePayload) {
    const instances = (payload.serviceDetail?.instances ?? []) as NonNullable<
      PageData["nodeContainers"]
    >;

    workspace = payload.workspace ?? null;
    tasks = payload.tasks ?? [];
    backups = payload.backups ?? [];
    serviceDetail = payload.serviceDetail ?? null;
    nodeContainers = instances;
    instanceLoadState = Object.fromEntries(
      instances.map((instance) => [
        instance.nodeId,
        instance.containers.length > 0 ? "loaded" : "idle",
      ]),
    ) as Record<string, "idle" | "loading" | "loaded" | "error">;
    instanceLoadError = {};
  }

  function createTab(file: WorkspaceFile): EditorTab {
    return {
      ...file,
      savedContent: file.content,
      dirty: false,
    };
  }

  let activeTab = $derived(
    openTabs.find((tab) => tab.path === activePath) ?? null,
  );
  let secondaryTab = $derived(
    openTabs.find((tab) => tab.path === secondaryPath) ?? null,
  );
  let focusedPath = $derived(
    focusedPane === "secondary" && splitEnabled ? secondaryPath : activePath,
  );
  let focusedTab = $derived(
    openTabs.find((tab) => tab.path === focusedPath) ?? null,
  );
  let editorRelatedFiles = $derived(
    Object.fromEntries(openTabs.map((tab) => [tab.path, tab.content])),
  );
  let canSave = $derived(
    Boolean(focusedTab && focusedTab.dirty && !focusedTab.readOnly && !saving),
  );
  let selectedNode = $derived(
    selectedNodePath ? findNode(fileTree, selectedNodePath) : null,
  );
  let recentTasks = $derived(
    tasks.filter((task) => isTaskRecent(task.createdAt)).slice(0, 4),
  );
  let workspaceNodesLabel = $derived(
    workspace?.nodes?.length ? workspace.nodes.join(", ") : "",
  );
  let backupCapability = $derived(
    serviceActionCapability(workspace?.actions, "backup"),
  );
  let dnsUpdateCapability = $derived(
    serviceActionCapability(workspace?.actions, "dnsUpdate"),
  );
  let caddySyncCapability = $derived(
    serviceActionCapability(workspace?.actions, "caddySync"),
  );
  let migrateCapability = $derived(
    serviceActionCapability(workspace?.actions, "migrate"),
  );
  let backupReason = $derived(
    backupCapability.enabled
      ? ""
      : capabilityReasonMessage(backupCapability.reasonCode, $messages),
  );
  let dnsUpdateReason = $derived(
    dnsUpdateCapability.enabled
      ? ""
      : capabilityReasonMessage(dnsUpdateCapability.reasonCode, $messages),
  );
  let caddySyncReason = $derived(
    caddySyncCapability.enabled
      ? ""
      : capabilityReasonMessage(caddySyncCapability.reasonCode, $messages),
  );
  let migrateReason = $derived(
    migrateCapability.enabled
      ? ""
      : capabilityReasonMessage(migrateCapability.reasonCode, $messages),
  );

  function fileUnavailableReason(file: WorkspaceFile | null) {
    if (!file?.unavailableReasonCode) {
      return "";
    }

    return capabilityReasonMessage(file.unavailableReasonCode, $messages);
  }

  function fileUnavailableDescription(file: WorkspaceFile | null) {
    if (!file || !isEncryptedFilePath(file.path)) {
      return "";
    }

    return file.unavailableReasonCode === "service_not_declared"
      ? $messages.services.files.encryptedUnavailableDeclared
      : $messages.services.files.encryptedUnavailableMissingSecrets;
  }

  let migrateSourceNodes = $derived(serviceDetail?.nodes ?? []);
  let hasMultipleInstanceNodes = $derived(nodeContainers.length > 1);
  let selectedInstanceEntry = $derived(
    nodeContainers.find(
      (instance) => instance.nodeId === selectedInstanceNode,
    ) ?? null,
  );
  let visibleNodeContainers = $derived(
    !hasMultipleInstanceNodes || selectedInstanceNode === "__all__"
      ? nodeContainers
      : nodeContainers.filter(
          (instance) => instance.nodeId === selectedInstanceNode,
        ),
  );
  let serviceOptions = $derived(
    data.services.map((service) => ({
      value: service.folder,
      label: service.displayName,
      secondary: service.folder,
    })),
  );

  function applyServiceSummaryState(payload: ServiceSummaryStatePayload) {
    workspace = payload.workspace ?? workspace;
    tasks = payload.tasks ?? tasks;
    backups = payload.backups ?? backups;
    serviceDetail = payload.serviceDetail ?? serviceDetail;

    if (payload.serviceDetail?.instances) {
      const existingByNode = new Map(
        nodeContainers.map((instance) => [instance.nodeId, instance] as const),
      );
      const nextLoadState: Record<
        string,
        "idle" | "loading" | "loaded" | "error"
      > = {};
      const nextLoadError: Record<string, string> = {};

      nodeContainers = payload.serviceDetail.instances.map((instance) => {
        const existing = existingByNode.get(instance.nodeId);
        const hasFreshContainers = instance.containers.length > 0;
        const changed =
          existing &&
          (existing.runtimeStatus !== instance.runtimeStatus ||
            existing.updatedAt !== instance.updatedAt ||
            existing.isDeclared !== instance.isDeclared);

        if (!existing || changed || hasFreshContainers) {
          nextLoadState[instance.nodeId] = hasFreshContainers
            ? "loaded"
            : "idle";
          return instance;
        }

        nextLoadState[instance.nodeId] =
          instanceLoadState[instance.nodeId] ?? "idle";
        if (nextLoadState[instance.nodeId] === "error") {
          nextLoadError[instance.nodeId] =
            instanceLoadError[instance.nodeId] ?? "";
        }

        return {
          ...instance,
          containers: existing.containers,
        };
      }) as NonNullable<PageData["nodeContainers"]>;

      instanceLoadState = nextLoadState;
      instanceLoadError = nextLoadError;
    }

    if (
      selectedInstanceNode !== "__all__" &&
      !nodeContainers.some(
        (instance) => instance.nodeId === selectedInstanceNode,
      )
    ) {
      selectedInstanceNode = "__all__";
    }
  }

  async function loadInstanceContainers(nodeId: string, force = false) {
    if (!workspace?.folder || !workspace.serviceName) {
      return;
    }
    const currentState = instanceLoadState[nodeId] ?? "idle";
    if (!force && (currentState === "loading" || currentState === "loaded")) {
      return;
    }

    instanceLoadState = { ...instanceLoadState, [nodeId]: "loading" };
    instanceLoadError = { ...instanceLoadError, [nodeId]: "" };

    try {
      const response = await fetch(
        `/services/${workspace.folder}/instances/${encodeURIComponent(nodeId)}`,
      );
      const payload = (await response.json()) as {
        instance?: ServiceInstanceDetail;
        error?: string;
      };
      if (!response.ok || !payload.instance) {
        throw new Error(payload.error ?? "Failed to load service instance.");
      }

      nodeContainers = nodeContainers.map((instance) =>
        instance.nodeId === nodeId ? payload.instance! : instance,
      ) as NonNullable<PageData["nodeContainers"]>;
      instanceLoadState = { ...instanceLoadState, [nodeId]: "loaded" };
      instanceLoadError = { ...instanceLoadError, [nodeId]: "" };
      errorMessage = "";
    } catch (loadError) {
      const message =
        loadError instanceof Error
          ? loadError.message
          : "Failed to load service instance.";
      instanceLoadState = { ...instanceLoadState, [nodeId]: "error" };
      instanceLoadError = { ...instanceLoadError, [nodeId]: message };
      errorMessage = message;
    }
  }

  $effect(() => {
    const nodesToLoad = visibleNodeContainers
      .filter(
        (instance) => (instanceLoadState[instance.nodeId] ?? "idle") === "idle",
      )
      .map((instance) => instance.nodeId);

    if (nodesToLoad.length === 0) {
      return;
    }

    for (const nodeId of nodesToLoad) {
      void loadInstanceContainers(nodeId);
    }
  });

  async function openFile(path: string) {
    try {
      const normalized = normalizeServiceRelativePath(path);
      const existing = openTabs.find((tab) => tab.path === normalized);
      const targetPane =
        splitEnabled && focusedPane === "secondary" ? "secondary" : "primary";
      if (existing) {
        selectedNodePath = normalized;
        if (targetPane === "secondary" && normalized !== activePath) {
          secondaryPath = normalized;
        } else {
          activePath = normalized;
        }
        return;
      }

      const response = await fetch(
        `/services/${workspace?.folder}/workspace?path=${encodeURIComponent(normalized)}`,
      );
      const payload = await response.json();
      if (!response.ok) {
        errorMessage = payload.error ?? "Failed to open file.";
        return;
      }

      openTabs = [...openTabs, createTab(payload.file as WorkspaceFile)];
      selectedNodePath = normalized;
      if (targetPane === "secondary" && normalized !== activePath) {
        secondaryPath = normalized;
      } else {
        activePath = normalized;
      }
      errorMessage = "";
    } catch (openError) {
      errorMessage =
        openError instanceof Error ? openError.message : "Failed to open file.";
    }
  }

  function closeTab(path: string) {
    const nextTabs = openTabs.filter((tab) => tab.path !== path);
    openTabs = nextTabs;
    if (activePath === path) {
      activePath = nextTabs[nextTabs.length - 1]?.path ?? "";
    }
    if (secondaryPath === path) {
      secondaryPath =
        nextTabs.find((tab) => tab.path !== activePath)?.path ?? "";
    }
    if (activePath && secondaryPath === activePath) {
      secondaryPath =
        nextTabs.find((tab) => tab.path !== activePath)?.path ?? "";
    }
  }

  function selectNode(path: string) {
    selectedNodePath = path;
    showRename = false;
    if (path) {
      renamePath = path;
    }
  }

  function updateTab(path: string, content: string) {
    openTabs = openTabs.map((tab) =>
      tab.path === path
        ? {
            ...tab,
            content,
            dirty: content !== tab.savedContent,
          }
        : tab,
    );
  }

  async function saveTab(path: string) {
    const tab = openTabs.find((item) => item.path === path) ?? null;
    if (!tab || !headRevision || tab.readOnly) {
      return;
    }

    saving = true;
    errorMessage = "";

    try {
      const response = await fetch(
        `/services/${workspace?.folder}/workspace/file`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            path: tab.path,
            content: tab.content,
            baseRevision: headRevision,
          }),
        },
      );
      const payload = (await response.json()) as {
        error?: string;
        file?: WorkspaceFile;
        write?: RepoWriteResult;
        workspace?: PageData["workspace"];
      };
      if (!response.ok || !payload.file || !payload.write) {
        throw new Error(payload.error ?? "Failed to save file.");
      }

      headRevision = payload.write.commitId;
      syncStatus = payload.write.syncStatus;
      syncError = payload.write.pushError;
      lastSuccessfulPullAt = payload.write.lastSuccessfulPullAt;
      openTabs = openTabs.map((item) =>
        item.path === tab.path
          ? {
              ...item,
              content: payload.file?.content ?? item.content,
              savedContent: payload.file?.content ?? item.content,
              dirty: false,
              size: payload.file?.size ?? item.size,
            }
          : item,
      );
      toast.success(`Saved ${tab.path}`);
    } catch (saveError) {
      errorMessage =
        saveError instanceof Error ? saveError.message : "Failed to save file.";
    } finally {
      saving = false;
    }
  }

  async function saveCurrentTab() {
    if (!focusedPath) {
      return;
    }

    await saveTab(focusedPath);
  }

  function toggleSplitEditor() {
    if (splitEnabled) {
      splitEnabled = false;
      secondaryPath = "";
      focusedPane = "primary";
      return;
    }

    splitEnabled = true;
    secondaryPath = openTabs.find((tab) => tab.path !== activePath)?.path ?? "";
  }

  function openInSecondary(path: string) {
    if (path === activePath) {
      return;
    }

    splitEnabled = true;
    secondaryPath = path;
    focusedPane = "secondary";
  }

  function focusPane(pane: "primary" | "secondary") {
    focusedPane = pane;
  }

  function resolveAlternatePath(paths: string[], excludedPath: string) {
    return paths.find((path) => path !== excludedPath) ?? "";
  }

  async function createFile() {
    if (!newFilePath.trim()) {
      return;
    }

    try {
      const normalized = normalizeServiceRelativePath(newFilePath);
      const targetPane =
        splitEnabled && focusedPane === "secondary" ? "secondary" : "primary";
      if (openTabs.some((tab) => tab.path === normalized)) {
        if (targetPane === "secondary" && normalized !== activePath) {
          secondaryPath = normalized;
        } else {
          activePath = normalized;
        }
        showNewFile = false;
        newFilePath = "";
        return;
      }

      saving = true;
      errorMessage = "";

      const response = await fetch(
        `/services/${workspace?.folder}/workspace/file`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            path: normalized,
            content: "",
            baseRevision: headRevision,
          }),
        },
      );
      const payload = (await response.json()) as {
        error?: string;
        file?: WorkspaceFile;
        write?: RepoWriteResult;
        workspace?: PageData["workspace"];
        fileTree?: ServiceFileNode[];
      };
      if (!response.ok || !payload.file || !payload.write) {
        throw new Error(payload.error ?? "Failed to create file.");
      }

      applyFsMutation({
        write: payload.write,
        workspace: payload.workspace,
        fileTree: payload.fileTree,
      });
      openTabs = [...openTabs, createTab(payload.file)];
      selectedNodePath = normalized;
      if (targetPane === "secondary" && normalized !== activePath) {
        secondaryPath = normalized;
      } else {
        activePath = normalized;
      }
      showNewFile = false;
      newFilePath = "";
      toast.success(`Created ${normalized}`);
    } catch (createError) {
      errorMessage =
        createError instanceof Error
          ? createError.message
          : "Failed to create file.";
    } finally {
      saving = false;
    }
  }

  async function createDirectory() {
    if (!newFolderPath.trim()) {
      return;
    }

    saving = true;
    errorMessage = "";

    try {
      const normalized = normalizeServiceRelativePath(newFolderPath);
      const response = await fetch(
        `/services/${workspace?.folder}/workspace/directories`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            path: normalized,
            baseRevision: headRevision,
          }),
        },
      );
      const payload = await response.json();
      if (!response.ok || !payload.write) {
        throw new Error(payload.error ?? "Failed to create folder.");
      }

      applyFsMutation(payload);
      selectedNodePath = normalized;
      showNewFolder = false;
      newFolderPath = "";
      toast.success(`Created folder ${normalized}`);
    } catch (directoryError) {
      errorMessage =
        directoryError instanceof Error
          ? directoryError.message
          : "Failed to create folder.";
    } finally {
      saving = false;
    }
  }

  async function renameNode() {
    if (!selectedNodePath || !renamePath.trim()) {
      return;
    }

    saving = true;
    errorMessage = "";

    try {
      const destination = normalizeServiceRelativePath(renamePath);
      const response = await fetch(
        `/services/${workspace?.folder}/workspace/path`,
        {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            sourcePath: selectedNodePath,
            destinationPath: destination,
            baseRevision: headRevision,
          }),
        },
      );
      const payload = await response.json();
      if (!response.ok || !payload.write) {
        throw new Error(payload.error ?? "Failed to rename path.");
      }

      applyFsMutation(payload);
      openTabs = openTabs.map((tab) => {
        if (
          tab.path === selectedNodePath ||
          tab.path.startsWith(`${selectedNodePath}/`)
        ) {
          const nextPath =
            destination + tab.path.slice(selectedNodePath.length);
          return { ...tab, path: nextPath };
        }
        return tab;
      });
      if (
        activePath === selectedNodePath ||
        activePath.startsWith(`${selectedNodePath}/`)
      ) {
        activePath = destination + activePath.slice(selectedNodePath.length);
      }
      if (
        secondaryPath === selectedNodePath ||
        secondaryPath.startsWith(`${selectedNodePath}/`)
      ) {
        secondaryPath =
          destination + secondaryPath.slice(selectedNodePath.length);
      }
      selectedNodePath = destination;
      renamePath = destination;
      showRename = false;
      toast.success(`Renamed to ${destination}`);
    } catch (renameError) {
      errorMessage =
        renameError instanceof Error
          ? renameError.message
          : "Failed to rename path.";
    } finally {
      saving = false;
    }
  }

  async function deleteNode() {
    if (
      !selectedNodePath ||
      !confirm(
        `Delete ${selectedNode?.isDir ? "folder" : "file"} ${selectedNodePath}?`,
      )
    ) {
      return;
    }

    saving = true;
    errorMessage = "";

    try {
      const deletedPath = selectedNodePath;
      const response = await fetch(
        `/services/${workspace?.folder}/workspace/path`,
        {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            path: deletedPath,
            baseRevision: headRevision,
          }),
        },
      );
      const payload = await response.json();
      if (!response.ok || !payload.write) {
        throw new Error(payload.error ?? "Failed to delete path.");
      }

      applyFsMutation(payload);
      const nextTabs = openTabs.filter(
        (tab) =>
          tab.path !== deletedPath && !tab.path.startsWith(`${deletedPath}/`),
      );
      openTabs = nextTabs;
      if (
        activePath === deletedPath ||
        activePath.startsWith(`${deletedPath}/`)
      ) {
        activePath = nextTabs[0]?.path ?? "";
      }
      if (
        secondaryPath === deletedPath ||
        secondaryPath.startsWith(`${deletedPath}/`)
      ) {
        secondaryPath = resolveAlternatePath(
          nextTabs.map((tab) => tab.path),
          activePath,
        );
      }
      if (secondaryPath === activePath) {
        secondaryPath = resolveAlternatePath(
          nextTabs.map((tab) => tab.path),
          activePath,
        );
      }
      selectedNodePath = "";
      showRename = false;
      toast.success(`Deleted ${deletedPath}`);
    } catch (deleteError) {
      errorMessage =
        deleteError instanceof Error
          ? deleteError.message
          : "Failed to delete path.";
    } finally {
      saving = false;
    }
  }

  function applyFsMutation(payload: {
    write: RepoWriteResult;
    workspace?: PageData["workspace"];
    fileTree?: ServiceFileNode[];
  }) {
    headRevision = payload.write.commitId;
    syncStatus = payload.write.syncStatus;
    syncError = payload.write.pushError;
    lastSuccessfulPullAt = payload.write.lastSuccessfulPullAt;
    workspace = payload.workspace ?? workspace;
    fileTree = payload.fileTree ?? fileTree;
  }

  async function refreshServiceSummary() {
    const response = await fetch(`/services/${workspace?.folder}/workspace`);
    const payload = await response.json();
    if (!response.ok) {
      throw new Error(payload.error ?? "Failed to refresh service summary.");
    }

    applyServiceSummaryState(
      payload as {
        workspace?: PageData["workspace"];
        tasks?: TaskSummary[];
        backups?: BackupSummary[];
        serviceDetail?: PageData["serviceDetail"];
      },
    );

    return payload as {
      workspace?: PageData["workspace"];
      tasks?: TaskSummary[];
      backups?: BackupSummary[];
      serviceDetail?: PageData["serviceDetail"];
    };
  }

  function startActionRefresh(taskId: string) {
    stopActionRefresh();

    stopActionRefreshHandle = startPolling(
      async () => {
        const payload = await refreshServiceSummary();
        const task = (payload.tasks ?? []).find(
          (entry) => entry.taskId === taskId,
        );
        if (!task) {
          return true;
        }

        return !isTerminalTaskStatus(task.status);
      },
      {
        intervalMs: 2500,
        errorIntervalMs: 4000,
        initialDelayMs: 1200,
      },
    );
  }

  function stopActionRefresh() {
    stopActionRefreshHandle?.();
    stopActionRefreshHandle = null;
  }

  function syncFileTreeIconTheme(root: HTMLElement) {
    fileTreeIconTheme = root.classList.contains("dark") ? "dark" : "light";
  }

  onMount(() => {
    const root = document.documentElement;
    syncFileTreeIconTheme(root);

    const themeObserver = new MutationObserver(() => {
      syncFileTreeIconTheme(root);
    });
    themeObserver.observe(root, {
      attributes: true,
      attributeFilter: ["class"],
    });

    const stopAutoRefresh = startPolling(
      async () => {
        await refreshServiceSummary();
      },
      {
        intervalMs: 5000,
      },
    );

    return () => {
      themeObserver.disconnect();
      stopAutoRefresh();
      stopActionRefresh();
    };
  });

  function isTaskRecent(createdAt: string) {
    const createdAtMs = Date.parse(createdAt);
    if (Number.isNaN(createdAtMs)) {
      return false;
    }
    return Date.now() - createdAtMs <= 24 * 60 * 60 * 1000;
  }

  function isTerminalTaskStatus(status: string) {
    return (
      status === "succeeded" || status === "failed" || status === "cancelled"
    );
  }

  async function triggerAction(
    action:
      | "deploy"
      | "update"
      | "stop"
      | "restart"
      | "backup"
      | "dns_update"
      | "caddy_sync",
  ) {
    if (action === "backup" && !backupCapability.enabled) {
      errorMessage = backupReason;
      return;
    }
    if (action === "dns_update" && !dnsUpdateCapability.enabled) {
      errorMessage = dnsUpdateReason;
      return;
    }
    if (action === "caddy_sync" && !caddySyncCapability.enabled) {
      errorMessage = caddySyncReason;
      return;
    }

    actionBusy = action;
    errorMessage = "";

    try {
      const response = await fetch(
        `/services/${workspace?.folder}/actions/${action}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
        },
      );
      const payload = (await response.json()) as ServiceActionResult & {
        error?: string;
        reasonCode?: string;
      };
      if (!response.ok || !payload.taskId) {
        throw new Error(
          actionErrorMessage(payload, $messages, `Failed to run ${action}.`),
        );
      }

      const newTask: TaskSummary = {
        taskId: payload.taskId,
        type: action,
        status: payload.status,
        serviceName: workspace?.serviceName ?? "",
        nodeId: "",
        createdAt: new Date().toISOString(),
      };
      tasks = [newTask, ...tasks].slice(0, 12);
      toast.success(`${action} queued as ${payload.taskId}`);
      startActionRefresh(payload.taskId);
    } catch (actionError) {
      errorMessage =
        actionError instanceof Error
          ? actionError.message
          : `Failed to run ${action}.`;
    } finally {
      actionBusy = "";
    }
  }

  async function triggerMigrate() {
    if (
      !workspace?.isDeclared ||
      !workspace?.serviceName ||
      !migrateSourceNode ||
      !migrateTargetNode.trim()
    ) {
      errorMessage = "Select a source node and enter a target node.";
      return;
    }
    if (!migrateCapability.enabled) {
      errorMessage = migrateReason;
      return;
    }
    actionBusy = "migrate";
    errorMessage = "";

    try {
      const response = await fetch(
        `/services/${workspace.folder}/actions/migrate`,
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            sourceNodeId: migrateSourceNode,
            targetNodeId: migrateTargetNode.trim(),
          }),
        },
      );
      const payload = (await response.json()) as ServiceActionResult & {
        error?: string;
        reasonCode?: string;
      };
      if (!response.ok || !payload.taskId) {
        throw new Error(
          actionErrorMessage(payload, $messages, "Failed to run migrate."),
        );
      }
      const newTask: TaskSummary = {
        taskId: payload.taskId,
        type: "migrate",
        status: payload.status,
        serviceName: workspace.serviceName,
        nodeId: migrateSourceNode,
        createdAt: new Date().toISOString(),
      };
      tasks = [newTask, ...tasks].slice(0, 12);
      toast.success(`migrate queued as ${payload.taskId}`);
      startActionRefresh(payload.taskId);
    } catch (actionError) {
      errorMessage =
        actionError instanceof Error
          ? actionError.message
          : "Failed to run migrate.";
    } finally {
      actionBusy = "";
    }
  }

  async function renameServiceRoot() {
    if (!renameServiceFolder.trim()) {
      return;
    }

    saving = true;
    errorMessage = "";

    try {
      const response = await fetch(`/services/${workspace?.folder}/root`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          folder: renameServiceFolder,
          baseRevision: headRevision,
        }),
      });
      const payload = await response.json();
      if (!response.ok || !payload.redirectTo) {
        throw new Error(payload.error ?? "Failed to rename service folder.");
      }

      window.location.href = payload.redirectTo;
    } catch (renameError) {
      errorMessage =
        renameError instanceof Error
          ? renameError.message
          : "Failed to rename service folder.";
      saving = false;
    }
  }

  async function deleteServiceRoot() {
    if (
      !workspace?.folder ||
      !confirm(`Delete service folder ${workspace.folder}?`)
    ) {
      return;
    }

    saving = true;
    errorMessage = "";

    try {
      const response = await fetch(`/services/${workspace.folder}/root`, {
        method: "DELETE",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          baseRevision: headRevision,
        }),
      });
      const payload = await response.json();
      if (!response.ok || !payload.redirectTo) {
        throw new Error(payload.error ?? "Failed to delete service folder.");
      }

      window.location.href = payload.redirectTo;
    } catch (deleteError) {
      errorMessage =
        deleteError instanceof Error
          ? deleteError.message
          : "Failed to delete service folder.";
      saving = false;
    }
  }

  function toggleDirectory(path: string) {
    const next = new Set(collapsedPaths);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    collapsedPaths = next;
  }

  async function selectService(folder: string) {
    serviceSwitchOpen = false;
    if (!folder || folder === workspace?.folder) {
      return;
    }
    await goto(`/services/${encodeURIComponent(folder)}`);
    await invalidateAll();
  }
</script>

<div class="page-shell-workbench flex min-h-[calc(100vh-72px)] flex-col">
  <div class="page-stack flex min-h-0 flex-1 flex-col">
    <Card>
      <CardHeader class="gap-3 py-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <Badge
            variant={runtimeStatusTone(workspace?.runtimeStatus ?? "unknown")}
          >
            {runtimeStatusLabel(workspace?.runtimeStatus ?? "", $messages)}
          </Badge>
        </div>

        <div class="max-w-2xl space-y-1">
          <Popover.Root bind:open={serviceSwitchOpen}>
            <Popover.Trigger class="inline-flex w-full">
              <button
                type="button"
                class="flex w-full items-center justify-between gap-2 rounded-md border border-input bg-background px-3 py-2 text-left text-sm shadow-xs transition-colors hover:bg-accent hover:text-accent-foreground"
              >
                <span
                  class="min-w-0 flex-1 truncate font-semibold text-foreground"
                >
                  {workspace?.displayName ?? $messages.services.selectService}
                </span>
                <ChevronsUpDownIcon class="size-4 shrink-0 opacity-50" />
              </button>
            </Popover.Trigger>
            <Popover.Content
              class="w-[min(92vw,28rem)] p-0"
              align="start"
              sideOffset={8}
            >
              <Command.Root>
                <Command.Input
                  placeholder={$messages.services.searchServicePlaceholder}
                />
                <Command.List>
                  <Command.Empty
                    >{$messages.services.noServicesFound}</Command.Empty
                  >
                  <Command.Group>
                    {#each serviceOptions as service (service.value)}
                      <Command.Item
                        value={`${service.label} ${service.secondary}`}
                        onSelect={() => {
                          void selectService(service.value);
                        }}
                      >
                        <div class="min-w-0">
                          <div class="truncate">{service.label}</div>
                          <div class="truncate text-xs text-muted-foreground">
                            {service.secondary}
                          </div>
                        </div>
                        <CheckIcon
                          class={cn(
                            "ml-auto size-4",
                            service.value !== workspace?.folder &&
                              "text-transparent",
                          )}
                        />
                      </Command.Item>
                    {/each}
                  </Command.Group>
                </Command.List>
              </Command.Root>
            </Popover.Content>
          </Popover.Root>

          <div class="truncate text-sm text-muted-foreground">
            {workspace?.folder ?? $messages.common.na}
          </div>
        </div>

        <div
          class="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground"
        >
          <div class="flex items-center gap-1.5">
            <span>{$messages.nodes.node}</span>
            <span class="font-medium text-foreground"
              >{workspaceNodesLabel || $messages.common.na}</span
            >
          </div>
          <div class="flex items-center gap-1.5">
            <span>{$messages.settings.repoSync.revision}</span>
            <span class="font-medium text-foreground"
              >{headRevision
                ? headRevision.slice(0, 12)
                : $messages.common.na}</span
            >
          </div>
          <div class="flex items-center gap-1.5">
            <span>{$messages.services.syncStatus}</span>
            <span class="font-medium text-foreground"
              >{syncStatus || $messages.status.unknown}</span
            >
          </div>
          <div class="flex items-center gap-1.5">
            <span>{$messages.services.lastPull}</span>
            <span class="font-medium text-foreground"
              >{lastSuccessfulPullAt || $messages.common.never}</span
            >
          </div>
        </div>

        {#if syncError}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.syncFailed}</AlertTitle>
            <AlertDescription>{syncError}</AlertDescription>
          </Alert>
        {/if}
      </CardHeader>
    </Card>

    {#if (nodeContainers ?? []).length > 0}
      <Card>
        <CardHeader class="gap-3">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <CardTitle class="section-title"
                >{$messages.services.instances}</CardTitle
              >
              <div class="text-sm text-muted-foreground">
                {$messages.services.containersByNode}
              </div>
            </div>

            {#if hasMultipleInstanceNodes}
              <Select type="single" bind:value={selectedInstanceNode as any}>
                <SelectTrigger class="w-[240px]">
                  {#if selectedInstanceNode === "__all__"}
                    <span>{$messages.services.allNodes}</span>
                  {:else}
                    <span>
                      {selectedInstanceNode}
                      {#if selectedInstanceEntry}
                        · {runtimeStatusLabel(
                          selectedInstanceEntry.runtimeStatus,
                          $messages,
                        )}
                      {/if}
                    </span>
                  {/if}
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__all__"
                    >{$messages.services.allNodes}</SelectItem
                  >
                  {#each nodeContainers ?? [] as instance}
                    <SelectItem value={instance.nodeId}>
                      {instance.nodeId} · {runtimeStatusLabel(
                        instance.runtimeStatus,
                        $messages,
                      )}
                    </SelectItem>
                  {/each}
                </SelectContent>
              </Select>
            {/if}
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          {#each visibleNodeContainers as instance, index}
            <section class="space-y-3">
              {#if hasMultipleInstanceNodes && selectedInstanceNode === "__all__"}
                <div class="relative flex items-center justify-center">
                  <div class="absolute inset-x-0 h-px bg-border/70"></div>
                  <div
                    class="relative flex items-center gap-2 bg-card px-3 text-sm font-medium text-foreground"
                  >
                    <span>{instance.nodeId}</span>
                    <Badge variant={runtimeStatusTone(instance.runtimeStatus)}
                      >{runtimeStatusLabel(
                        instance.runtimeStatus,
                        $messages,
                      )}</Badge
                    >
                  </div>
                  <div class="h-px flex-1 bg-border/70"></div>
                </div>
              {/if}

              {#if instanceLoadState[instance.nodeId] === "loading"}
                <div
                  class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground"
                >
                  {$messages.common.loadingWithDots}
                </div>
              {:else if instanceLoadState[instance.nodeId] === "error"}
                <div
                  class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground"
                >
                  <div>
                    {instanceLoadError[instance.nodeId] ||
                      $messages.error.loadFailed}
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    class="mt-3"
                    onclick={() =>
                      loadInstanceContainers(instance.nodeId, true)}
                  >
                    <RefreshCcw class="mr-2 size-4" />{$messages.common.refresh}
                  </Button>
                </div>
              {:else if instance.containers.length > 0}
                <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {#each instance.containers as container}
                    <a
                      href="/nodes/{instance.nodeId}/docker/containers/{encodeURIComponent(
                        container.containerId,
                      )}"
                      class="block rounded-md border border-border/60 bg-background px-3 py-2 transition-colors hover:bg-accent/40"
                    >
                      <div class="flex items-center justify-between gap-2">
                        <div class="font-medium">{container.name}</div>
                        <Badge variant={runtimeStatusTone(container.state)}
                          >{runtimeStatusLabel(
                            container.state,
                            $messages,
                          )}</Badge
                        >
                      </div>
                      <div class="mt-1 text-xs text-muted-foreground">
                        {container.image}
                      </div>
                      <div class="mt-1 text-[11px] text-muted-foreground/80">
                        {container.composeProject}/{container.composeService}
                      </div>
                    </a>
                  {/each}
                </div>
              {:else}
                <div
                  class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground"
                >
                  {$messages.services.noContainersOnNode}
                </div>
              {/if}
            </section>
          {/each}
        </CardContent>
      </Card>
    {/if}

    {#if errorMessage}
      <Alert variant="destructive">
        <AlertTitle>{$messages.error.workspaceError}</AlertTitle>
        <AlertDescription>{errorMessage}</AlertDescription>
      </Alert>
    {/if}

    <div
      class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)] 2xl:grid-cols-[300px_minmax(0,1fr)_280px]"
    >
      <Card class="flex min-h-0 min-w-0 flex-col">
        <CardHeader class="section-header border-b">
          <CardTitle class="section-title"
            >{$messages.services.files.title}</CardTitle
          >
          <div class="flex flex-wrap items-center gap-2">
            <Popover.Root bind:open={showNewFile}>
              <Popover.Trigger class="inline-flex">
                {#snippet child({ props: triggerProps })}
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    {...triggerProps}
                  >
                    <FilePlus class="mr-2 size-4" />{$messages.common.newFile}
                  </Button>
                {/snippet}
              </Popover.Trigger>
              <Popover.Content class="w-80" sideOffset={8}>
                <div class="space-y-3">
                  <Input
                    bind:value={newFilePath}
                    placeholder="config/new-file.yaml"
                  />
                  <div class="flex items-center justify-between gap-3">
                    <p class="text-xs text-muted-foreground">
                      {$messages.common.parentsAutoCreated}
                    </p>
                    <Button
                      type="button"
                      size="sm"
                      onclick={createFile}
                      disabled={saving}>{$messages.common.create}</Button
                    >
                  </div>
                </div>
              </Popover.Content>
            </Popover.Root>

            <Popover.Root bind:open={showNewFolder}>
              <Popover.Trigger class="inline-flex">
                {#snippet child({ props: triggerProps })}
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    {...triggerProps}
                  >
                    <FolderPlus class="mr-2 size-4" />{$messages.common
                      .newFolder}
                  </Button>
                {/snippet}
              </Popover.Trigger>
              <Popover.Content class="w-80" sideOffset={8}>
                <div class="space-y-3">
                  <Input
                    bind:value={newFolderPath}
                    placeholder="config/snippets"
                  />
                  <div class="flex items-center justify-between gap-3">
                    <p class="text-xs text-muted-foreground">
                      {$messages.common.trackedWithGitkeep}
                    </p>
                    <Button
                      type="button"
                      size="sm"
                      onclick={createDirectory}
                      disabled={saving}>{$messages.common.create}</Button
                    >
                  </div>
                </div>
              </Popover.Content>
            </Popover.Root>

            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={() => {
                showRename = !showRename;
                renamePath = selectedNodePath;
              }}
              disabled={!selectedNodePath || saving}
            >
              <Pencil class="mr-2 size-4" />{$messages.common.rename}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onclick={() => (showDeleteDialog = true)}
              disabled={!selectedNodePath || saving}
            >
              <Trash2 class="mr-2 size-4" />{$messages.common.delete}
            </Button>
          </div>
        </CardHeader>

        {#if showRename}
          <div class="border-b px-4 py-3 text-sm">
            <div class="space-y-3">
              <Input bind:value={renamePath} placeholder="new/path.yaml" />
              <div class="flex justify-end">
                <Button
                  type="button"
                  size="sm"
                  onclick={renameNode}
                  disabled={!selectedNodePath || saving}
                  >{$messages.common.apply}</Button
                >
              </div>
            </div>
          </div>
        {/if}

        <div class="min-h-0 flex-1 overflow-auto px-2 py-3">
          <ServiceFileTree
            nodes={fileTree}
            {activePath}
            selectedPath={selectedNodePath}
            {collapsedPaths}
            iconTheme={fileTreeIconTheme}
            onOpenFile={openFile}
            onSelectNode={selectNode}
            onToggle={toggleDirectory}
          />
        </div>
      </Card>

      <Card class="flex min-h-0 min-w-0 flex-col">
        <CardHeader class="border-b">
          <div class="section-header">
            <CardTitle class="section-title"
              >{$messages.services.files.editor}</CardTitle
            >
            <div class="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onclick={toggleSplitEditor}
              >
                <Columns2 class="mr-2 size-4" />
                {splitEnabled
                  ? $messages.services.files.singleView
                  : $messages.services.files.splitView}
              </Button>
              <Button
                type="button"
                size="sm"
                onclick={saveCurrentTab}
                disabled={!canSave}
              >
                <Save class="mr-2 size-4" />
                {$messages.common.save}
              </Button>
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            {#each openTabs as tab}
              <div
                class="inline-flex items-center gap-2 rounded-md border bg-background px-3 py-1.5 text-sm"
                class:bg-secondary={tab.path === activePath ||
                  (splitEnabled && tab.path === secondaryPath)}
              >
                <button
                  type="button"
                  class="max-w-48 truncate"
                  onclick={() => {
                    activePath = tab.path;
                    focusPane("primary");
                  }}
                >
                  {tab.path}
                  {#if tab.dirty}*{/if}
                </button>
                {#if splitEnabled && tab.path !== activePath}
                  <button
                    type="button"
                    class="text-xs text-muted-foreground hover:text-foreground"
                    onclick={() => openInSecondary(tab.path)}
                    title={$messages.services.files.openInSplit}
                  >
                    <Columns2 class="size-3.5" />
                  </button>
                {/if}
                <button
                  type="button"
                  class="text-xs text-muted-foreground hover:text-foreground"
                  onclick={() => closeTab(tab.path)}>x</button
                >
              </div>
            {/each}
          </div>
        </CardHeader>

        {#if activeTab || (splitEnabled && secondaryTab)}
          <div class="grid min-h-0 flex-1" class:grid-cols-2={splitEnabled}>
            <div
              class="flex min-h-0 min-w-0 flex-col overflow-hidden"
              class:border-r={splitEnabled}
              onfocusin={() => focusPane("primary")}
            >
              <div
                class="flex items-center justify-between border-b px-3 py-2 text-xs text-muted-foreground"
              >
                <span class="truncate">{activeTab?.path ?? ""}</span>
              </div>
              {#if activeTab}
                <div class="min-h-0 flex-1">
                  {#key `primary:${activePath}`}
                    {#if activeTab.unavailableReasonCode}
                      <div class="flex h-full items-center justify-center px-6 py-8">
                        <div class="max-w-md rounded-xl border border-dashed border-border/70 bg-muted/20 p-6 text-center">
                          <div class="mx-auto mb-4 inline-flex size-10 items-center justify-center rounded-full bg-background text-muted-foreground">
                            <Lock class="size-5" />
                          </div>
                          <div class="text-sm font-medium">
                            {$messages.services.files.encryptedUnavailableTitle}
                          </div>
                          <div class="mt-2 text-sm text-muted-foreground">
                            {fileUnavailableReason(activeTab)}
                          </div>
                          <div class="mt-2 text-sm text-muted-foreground">
                            {fileUnavailableDescription(activeTab)}
                          </div>
                        </div>
                      </div>
                    {:else}
                      <CodeEditor
                        path={activeTab.path}
                        value={activeTab.content}
                        relatedFiles={editorRelatedFiles}
                        readOnly={activeTab.readOnly}
                        onchange={({ value }) => updateTab(activeTab.path, value)}
                        onsave={() => saveTab(activeTab.path)}
                      />
                    {/if}
                  {/key}
                </div>
              {:else}
                <div
                  class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground"
                >
                  {$messages.services.files.openFileToEdit}
                </div>
              {/if}
            </div>

            {#if splitEnabled}
              <div
                class="flex min-h-0 min-w-0 flex-col overflow-hidden"
                onfocusin={() => focusPane("secondary")}
              >
                <div
                  class="flex items-center justify-between border-b px-3 py-2 text-xs text-muted-foreground"
                >
                  <span class="truncate">{secondaryTab?.path ?? ""}</span>
                </div>
                {#if secondaryTab}
                  <div class="min-h-0 flex-1">
                    {#key `secondary:${secondaryPath}`}
                      {#if secondaryTab.unavailableReasonCode}
                        <div class="flex h-full items-center justify-center px-6 py-8">
                          <div class="max-w-md rounded-xl border border-dashed border-border/70 bg-muted/20 p-6 text-center">
                            <div class="mx-auto mb-4 inline-flex size-10 items-center justify-center rounded-full bg-background text-muted-foreground">
                              <Lock class="size-5" />
                            </div>
                            <div class="text-sm font-medium">
                              {$messages.services.files.encryptedUnavailableTitle}
                            </div>
                            <div class="mt-2 text-sm text-muted-foreground">
                              {fileUnavailableReason(secondaryTab)}
                            </div>
                            <div class="mt-2 text-sm text-muted-foreground">
                              {fileUnavailableDescription(secondaryTab)}
                            </div>
                          </div>
                        </div>
                      {:else}
                        <CodeEditor
                          path={secondaryTab.path}
                          value={secondaryTab.content}
                          relatedFiles={editorRelatedFiles}
                          readOnly={secondaryTab.readOnly}
                          onchange={({ value }) =>
                            updateTab(secondaryTab.path, value)}
                          onsave={() => saveTab(secondaryTab.path)}
                        />
                      {/if}
                    {/key}
                  </div>
                {:else}
                  <div
                    class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground"
                  >
                    {$messages.services.files.openFileToEditSplit}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {:else}
          <div
            class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground"
          >
            {$messages.services.files.openFileToEdit}
          </div>
        {/if}
      </Card>

      <section
        class="grid min-h-0 min-w-0 gap-4 xl:col-span-2 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)] 2xl:col-span-1 2xl:grid-cols-1"
      >
        <Card class="xl:col-span-2 2xl:col-span-1">
          <CardHeader class="section-header">
            <CardTitle class="section-title"
              >{$messages.services.operations.title}</CardTitle
            >
          </CardHeader>
          <CardContent class="space-y-4">
            <div class="grid gap-2">
              <Button
                type="button"
                onclick={() => triggerAction("deploy")}
                disabled={!!actionBusy || !workspace?.isDeclared}
              >
                <Play class="mr-2 size-4" />{$messages.services.operations
                  .deploy}
              </Button>
              <Button
                type="button"
                variant="outline"
                onclick={() => triggerAction("update")}
                disabled={!!actionBusy || !workspace?.isDeclared}
              >
                <Upload class="mr-2 size-4" />{$messages.services.operations
                  .update}
              </Button>
              <Button
                type="button"
                variant="outline"
                onclick={() => triggerAction("restart")}
                disabled={!!actionBusy || !workspace?.isDeclared}
              >
                <RefreshCcw class="mr-2 size-4" />{$messages.services.operations
                  .restart}
              </Button>
              <Button
                type="button"
                variant="outline"
                onclick={() => triggerAction("stop")}
                disabled={!!actionBusy || !workspace?.isDeclared}
              >
                <Square class="mr-2 size-4" />{$messages.services.operations
                  .stop}
              </Button>
            </div>

            <Collapsible bind:open={advancedOperationsOpen}>
              <CollapsibleTrigger
                class="group flex w-full items-center gap-3 py-1 text-xs text-muted-foreground hover:text-foreground"
              >
                <div
                  class="h-px flex-1 bg-border/70 transition-colors group-hover:bg-border"
                ></div>
                <span
                  >{advancedOperationsOpen
                    ? $messages.services.collapse
                    : $messages.services.expand}</span
                >
                <div
                  class="h-px flex-1 bg-border/70 transition-colors group-hover:bg-border"
                ></div>
              </CollapsibleTrigger>
              <CollapsibleContent>
                <div class="grid gap-2 pt-3">
                  <DisabledReasonTooltip reason={backupReason}>
                    <Button
                      type="button"
                      variant="outline"
                      onclick={() => triggerAction("backup")}
                      disabled={!!actionBusy ||
                        !workspace?.isDeclared ||
                        !backupCapability.enabled}
                    >
                      <Wrench class="mr-2 size-4" />{$messages.services
                        .operations.backup}
                    </Button>
                  </DisabledReasonTooltip>
                  <DisabledReasonTooltip reason={dnsUpdateReason}>
                    <Button
                      type="button"
                      variant="outline"
                      onclick={() => triggerAction("dns_update")}
                      disabled={!!actionBusy ||
                        !workspace?.isDeclared ||
                        !dnsUpdateCapability.enabled}
                    >
                      <Upload class="mr-2 size-4" />{$messages.services
                        .operations.dnsUpdate}
                    </Button>
                  </DisabledReasonTooltip>
                  <DisabledReasonTooltip reason={caddySyncReason}>
                    <Button
                      type="button"
                      variant="outline"
                      onclick={() => triggerAction("caddy_sync")}
                      disabled={!!actionBusy ||
                        !workspace?.isDeclared ||
                        !(workspace?.nodes?.length ?? 0) ||
                        !caddySyncCapability.enabled}
                    >
                      <Copy class="mr-2 size-4" />{$messages.services.operations
                        .syncCaddy}
                    </Button>
                  </DisabledReasonTooltip>
                  <div
                    class="space-y-3 rounded-lg border border-border/60 bg-muted/20 p-3"
                  >
                    <div class="text-sm font-medium">
                      {$messages.services.operations.migrate.title}
                    </div>
                    <Select type="single" bind:value={migrateSourceNode as any}>
                      <SelectTrigger>
                        <span
                          >{migrateSourceNode ||
                            $messages.services.operations.migrate
                              .selectSource}</span
                        >
                      </SelectTrigger>
                      <SelectContent>
                        {#each migrateSourceNodes as nodeId}
                          <SelectItem value={nodeId}>{nodeId}</SelectItem>
                        {/each}
                      </SelectContent>
                    </Select>
                    <Input
                      bind:value={migrateTargetNode}
                      placeholder={$messages.services.operations.migrate
                        .targetNodeId}
                    />
                    <DisabledReasonTooltip reason={migrateReason}>
                      <Button
                        type="button"
                        variant="outline"
                        onclick={triggerMigrate}
                        disabled={!!actionBusy ||
                          !workspace?.isDeclared ||
                          !migrateSourceNode ||
                          !migrateTargetNode.trim() ||
                          !migrateCapability.enabled}
                      >
                        <RefreshCcw class="mr-2 size-4" />{$messages.services
                          .operations.migrate.migrate}
                      </Button>
                    </DisabledReasonTooltip>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    onclick={() => {
                      showServiceRename = !showServiceRename;
                      renameServiceFolder = workspace?.folder ?? "";
                    }}
                    disabled={saving}
                  >
                    <Pencil class="mr-2 size-4" />{$messages.services.operations
                      .renameFolder}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    class="border-destructive text-destructive hover:bg-destructive/10 hover:text-destructive"
                    onclick={deleteServiceRoot}
                    disabled={saving}
                  >
                    <Trash2 class="mr-2 size-4" />{$messages.services.operations
                      .deleteService}
                  </Button>
                </div>
              </CollapsibleContent>
            </Collapsible>

            {#if showServiceRename}
              <div class="space-y-3 border-t pt-4">
                <Input
                  bind:value={renameServiceFolder}
                  placeholder="new-service-folder"
                />
                <div class="flex justify-end">
                  <Button
                    type="button"
                    size="sm"
                    onclick={renameServiceRoot}
                    disabled={saving}>{$messages.common.apply}</Button
                  >
                </div>
              </div>
            {/if}

            {#if !workspace?.hasMeta}
              <div class="empty-state px-3 py-4">
                {$messages.services.addMetaToDeclare}
              </div>
            {:else if !workspace?.isDeclared}
              <div class="empty-state px-3 py-4">
                {$messages.services.fixMetaUntilAccepted}
              </div>
            {/if}
          </CardContent>
        </Card>

        <Card>
          <CardHeader class="section-header">
            <div class="section-heading">
              <CardTitle class="section-title"
                >{$messages.services.recentTasks}</CardTitle
              >
            </div>
            {#if workspace?.serviceName}
              <a
                class="text-sm text-muted-foreground transition-colors hover:text-foreground"
                href={`/tasks?serviceName=${encodeURIComponent(workspace.serviceName)}`}
              >
                {$messages.common.viewAll}
              </a>
            {/if}
          </CardHeader>
          <CardContent class="space-y-3">
            {#each recentTasks as task}
              <TaskItem {task} showService={false} />
            {/each}
            {#if !recentTasks.length}
              <div class="empty-state px-3 py-6">{$messages.tasks.noTasks}</div>
            {/if}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle class="section-title"
              >{$messages.services.recentBackups}</CardTitle
            >
          </CardHeader>
          <CardContent class="space-y-2">
            {#each backups.slice(0, 6) as backup}
              <div class="list-row-compact">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="font-medium">{backup.dataName}</div>
                    <div class="text-xs text-muted-foreground">
                      {backup.backupId}
                    </div>
                  </div>
                  <Badge variant={taskStatusTone(backup.status)}
                    >{taskStatusLabel(backup.status, $messages)}</Badge
                  >
                </div>
                <div class="mt-2 text-xs text-muted-foreground">
                  {formatTimestamp(backup.finishedAt || backup.startedAt)}
                </div>
              </div>
            {/each}
            {#if !backups.length}
              <div class="empty-state px-3 py-6">
                {$messages.backups.noBackups}
              </div>
            {/if}
          </CardContent>
        </Card>
      </section>
    </div>

    <Dialog bind:open={showDeleteDialog}>
      <DialogOverlay />
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle
            >{$messages.common.delete}
            {selectedNode?.isDir
              ? $messages.common.folder
              : $messages.common.file}?</DialogTitle
          >
          <DialogDescription>
            {selectedNode?.isDir
              ? $messages.services.deleteFolderConfirm
              : $messages.services.deleteFileConfirm}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onclick={() => (showDeleteDialog = false)}
            >{$messages.common.cancel}</Button
          >
          <Button
            type="button"
            variant="destructive"
            onclick={() => {
              showDeleteDialog = false;
              deleteNode();
            }}
            disabled={saving}>{$messages.common.delete}</Button
          >
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
