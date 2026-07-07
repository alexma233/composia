<script lang="ts">
  import { Search } from "@lucide/svelte";
  import type { Snippet } from "svelte";

  import { Alert, AlertDescription } from "$lib/components/ui/alert";
  import { Badge } from "$lib/components/ui/badge";
  import { Button } from "$lib/components/ui/button";
  import {
    Card,
    CardContent,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import { Input } from "$lib/components/ui/input";
  import {
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNextButton,
    PaginationPrevButton,
  } from "$lib/components/ui/pagination";
  import Spinner from "$lib/components/ui/spinner/spinner.svelte";
  import { getMessages } from "$lib/i18n";

  const messages = getMessages();

  interface Props {
    title: string;
    subtitle: string;
    backHref: string;
    backLabel: string;
    totalCount: number;
    pageSize: number;
    itemCount: number;
    totalPages: number;
    ready: boolean;
    loading: boolean;
    error: string | null;
    searchId: string;
    searchPlaceholder: string;
    loadingText: string;
    emptyText: string;
    noResultsText: string;
    countSummary?: string;
    searchQuery: string;
    currentPage: number;
    onSearchInput: () => void;
    onClearSearch: () => void;
    onRefresh: () => void | Promise<void>;
    children?: Snippet;
  }

  let {
    title,
    subtitle,
    backHref,
    backLabel,
    totalCount,
    pageSize,
    itemCount,
    totalPages,
    ready,
    loading,
    error,
    searchId,
    searchPlaceholder,
    loadingText,
    emptyText,
    noResultsText,
    countSummary,
    searchQuery = $bindable(),
    currentPage = $bindable(),
    onSearchInput,
    onClearSearch,
    onRefresh,
    children,
  }: Props = $props();
</script>

<Card>
  <CardHeader>
    <div class="page-header">
      <div class="page-heading">
        <CardTitle class="page-title" level="1">{title}</CardTitle>
        <p class="page-description">
          {subtitle}
          {#if !loading}
            <Badge variant="outline" class="ml-2">{totalCount}</Badge>
          {/if}
        </p>
      </div>
      <a
        href={backHref}
        class="text-sm text-muted-foreground transition-colors hover:text-foreground"
      >
        {backLabel}
      </a>
    </div>

    <div class="flex items-center gap-3">
      <div class="relative max-w-sm flex-1">
        <label class="sr-only" for={searchId}>{searchPlaceholder}</label>
        <Search
          class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          id={searchId}
          type="text"
          placeholder={searchPlaceholder}
          aria-label={searchPlaceholder}
          class="pl-9"
          bind:value={searchQuery}
          oninput={onSearchInput}
        />
      </div>
      {#if searchQuery}
        <Button variant="ghost" size="sm" onclick={onClearSearch}>
          {$messages.common.cancel}
        </Button>
      {/if}
      <Button
        variant="outline"
        size="sm"
        onclick={() => void onRefresh()}
        disabled={loading || !ready}
      >
        {#if loading}{$messages.common.loading}...{:else}{$messages.common
            .refresh}{/if}
      </Button>
    </div>
  </CardHeader>
  <CardContent aria-busy={loading}>
    {#if error}
      <Alert variant="destructive">
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    {:else if loading}
      <div
        class="flex min-h-80 items-center justify-center"
        role="status"
        aria-live="polite"
      >
        <div class="flex items-center gap-3 text-sm text-muted-foreground">
          <Spinner />
          <span>{loadingText}</span>
        </div>
      </div>
    {:else if itemCount > 0}
      {@render children?.()}

      {#if countSummary}
        <div class="mt-3 text-center text-xs text-muted-foreground">
          {countSummary}
        </div>
      {/if}

      {#if totalPages > 1}
        <div class="mt-6">
          <Pagination
            count={totalCount}
            perPage={pageSize}
            bind:page={currentPage}
          >
            {#snippet children({ pages, currentPage })}
              <PaginationContent>
                <PaginationItem>
                  <PaginationPrevButton />
                </PaginationItem>

                {#each pages as page (page.key)}
                  {#if page.type === "ellipsis"}
                    <PaginationItem>
                      <PaginationEllipsis />
                    </PaginationItem>
                  {:else}
                    <PaginationItem>
                      <PaginationLink
                        {page}
                        isActive={currentPage === page.value}
                      />
                    </PaginationItem>
                  {/if}
                {/each}

                <PaginationItem>
                  <PaginationNextButton />
                </PaginationItem>
              </PaginationContent>
            {/snippet}
          </Pagination>
        </div>
      {/if}
    {:else if searchQuery}
      <div class="empty-state">
        {noResultsText}
      </div>
    {:else}
      <div class="empty-state">{emptyText}</div>
    {/if}
  </CardContent>
</Card>
