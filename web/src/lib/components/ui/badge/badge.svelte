<script lang="ts" module>
  import { cva, type VariantProps } from 'class-variance-authority';

  export const badgeVariants = cva(
    'inline-flex items-center rounded-md border px-2 py-0.5 text-xs font-medium transition-colors',
    {
      variants: {
        variant: {
          default: 'border-primary/20 bg-primary/10 text-primary',
          secondary: 'border-border bg-secondary text-secondary-foreground',
          success: 'border-emerald-600/20 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300',
          warning: 'border-amber-600/20 bg-amber-500/10 text-amber-700 dark:text-amber-300',
          danger: 'border-rose-600/20 bg-rose-500/10 text-rose-700 dark:text-rose-300',
          info: 'border-sky-600/20 bg-sky-500/10 text-sky-700 dark:text-sky-300',
          outline: 'border-border text-foreground'
        }
      },
      defaultVariants: {
        variant: 'default'
      }
    }
  );

  export type Variant = VariantProps<typeof badgeVariants>['variant'];
</script>

<script lang="ts">
  import { cn } from '$lib/utils';

  interface Props {
    variant?: Variant;
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { variant = 'default', class: className = '', children, ...restProps }: Props = $props();
</script>

<span class={cn(badgeVariants({ variant }), className)} {...restProps}>
  {@render children?.()}
</span>
