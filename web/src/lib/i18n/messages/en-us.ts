export const enUS = {
  app: {
    name: 'Composia',
    subtitle: 'Service-first self-hosted operations'
  },
  nav: {
    overview: 'Overview',
    services: 'Services',
    nodes: 'Nodes',
    tasks: 'Tasks',
    backups: 'Backups',
    repo: 'Repo'
  },
  preferences: {
    theme: 'Theme',
    accent: 'Accent',
    locale: 'Locale',
    light: 'Light',
    dark: 'Dark',
    system: 'System'
  }
} as const;

export type Dictionary = typeof enUS;
