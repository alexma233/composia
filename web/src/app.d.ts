declare global {
  namespace App {
    interface Locals {
      user: {
        name: string;
      } | null;
    }
  }
}

export {};
