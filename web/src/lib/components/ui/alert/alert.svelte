<script lang="ts">
  import { cva, type VariantProps } from 'class-variance-authority';

  import { cn } from '$lib/utils';

  export const alertVariants = cva('relative w-full rounded-lg border px-4 py-3 text-sm', {
    variants: {
      variant: {
        default: 'border-border bg-card text-card-foreground',
        destructive: 'border-destructive/20 bg-destructive/10 text-destructive dark:border-destructive/30'
      }
    },
    defaultVariants: {
      variant: 'default'
    }
  });

  type Variant = VariantProps<typeof alertVariants>['variant'];

  interface Props {
    variant?: Variant;
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let { variant = 'default', class: className = '', children, ...restProps }: Props = $props();
</script>

<div role="alert" class={cn(alertVariants({ variant }), className)} {...restProps}>
  {@render children?.()}
</div>
