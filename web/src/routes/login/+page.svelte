<script lang="ts">
  import type { ActionData, PageData } from "./$types";

  import { messages } from "$lib/i18n";
  import { Button } from "$lib/components/ui/button";
  import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
  } from "$lib/components/ui/card";
  import { Input } from "$lib/components/ui/input";
  import { Label } from "$lib/components/ui/label";

  interface Props {
    data: PageData;
    form?: ActionData;
  }

  let { data, form }: Props = $props();
</script>

<svelte:head>
  <title>{$messages.auth.login.pageTitle}</title>
</svelte:head>

<div class="flex min-h-screen items-center justify-center px-4 py-10">
  <Card class="w-full max-w-md border-border/70 bg-background/95 shadow-lg backdrop-blur">
    <CardHeader class="space-y-2 text-center">
      <CardTitle class="text-2xl font-semibold tracking-tight">{$messages.auth.login.title}</CardTitle>
      <CardDescription>{$messages.auth.login.description}</CardDescription>
    </CardHeader>
    <CardContent>
      {#if !data.ready}
        <div class="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
          {data.error}
        </div>
      {:else}
        <form method="POST" class="space-y-4">
          <input type="hidden" name="next" value={data.next} />

          <div class="space-y-2">
            <Label for="username">{$messages.auth.login.username}</Label>
            <Input id="username" name="username" type="text" autocomplete="username" required />
          </div>

          <div class="space-y-2">
            <Label for="password">{$messages.auth.login.password}</Label>
            <Input
              id="password"
              name="password"
              type="password"
              autocomplete="current-password"
              required
            />
          </div>

          {#if form?.invalid}
            <div class="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {$messages.auth.login.invalidCredentials}
            </div>
          {:else if form?.error}
            <div class="rounded-lg border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {form.error}
            </div>
          {/if}

          <Button type="submit" class="w-full">{$messages.auth.login.submit}</Button>
        </form>
      {/if}
    </CardContent>
  </Card>
</div>
