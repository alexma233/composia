declare module "envfile" {
  export function parse(source: string): Record<string, string>;
}
