export const enUS = {
  app: {
    name: 'Composia',
    subtitle: 'Service-first self-hosted operations'
  },
  nav: {
    overview: 'Dashboard',
    services: 'Services',
    nodes: 'Nodes',
    tasks: 'Tasks',
    backups: 'Backups',
    settings: 'Settings'
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
