<script lang="ts">
  import type { ActionData, PageData } from "./$types";

  import { messages } from "$lib/i18n";
  import { Alert, AlertDescription, AlertTitle } from "$lib/components/ui/alert";
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
        <Alert variant="destructive">
          <AlertTitle>{$messages.error.loadFailed}</AlertTitle>
          <AlertDescription>{data.error}</AlertDescription>
        </Alert>
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
            <Alert variant="destructive">
              <AlertTitle>{$messages.auth.login.invalidCredentials}</AlertTitle>
            </Alert>
          {:else if form?.error}
            <Alert variant="destructive">
              <AlertTitle>{$messages.auth.login.invalidCredentials}</AlertTitle>
              <AlertDescription>{form.error}</AlertDescription>
            </Alert>
          {/if}

          <Button type="submit" class="w-full">{$messages.auth.login.submit}</Button>
        </form>
      {/if}
    </CardContent>
  </Card>
</div>
