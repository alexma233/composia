<script lang="ts">
  import { cva, type VariantProps } from 'class-variance-authority';

  import { cn } from '$lib/utils';

  export const buttonVariants = cva(
    'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md border text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:size-4 shrink-0',
    {
      variants: {
        variant: {
          default: 'border-primary bg-primary text-primary-foreground shadow-xs hover:bg-primary/90',
          secondary:
            'border-secondary bg-secondary text-secondary-foreground shadow-xs hover:bg-secondary/80',
          outline: 'border-border bg-background shadow-xs hover:bg-accent hover:text-accent-foreground',
          ghost: 'border-transparent hover:bg-accent hover:text-accent-foreground',
          destructive:
            'border-destructive bg-destructive text-destructive-foreground shadow-xs hover:bg-destructive/90'
        },
        size: {
          default: 'h-9 px-4 py-2',
          sm: 'h-8 rounded-md px-3 text-xs',
          lg: 'h-10 rounded-md px-6',
          icon: 'size-9 p-0'
        }
      },
      defaultVariants: {
        variant: 'default',
        size: 'default'
      }
    }
  );

  type Variant = VariantProps<typeof buttonVariants>['variant'];
  type Size = VariantProps<typeof buttonVariants>['size'];

  interface Props {
    variant?: Variant;
    size?: Size;
    type?: 'button' | 'submit' | 'reset';
    class?: string;
    children?: import('svelte').Snippet;
    [key: string]: unknown;
  }

  let {
    variant = 'default',
    size = 'default',
    type = 'button',
    class: className = '',
    children,
    ...restProps
  }: Props = $props();
</script>

<button type={type} class={cn(buttonVariants({ variant, size }), className)} {...restProps}>
  {@render children?.()}
</button>
