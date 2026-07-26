import type { NavigationItem } from "@/config/navigation";

export interface RbacSubject {
  permissions: string[];
}

export function hasPermission(subject: RbacSubject, permission?: string) {
  if (!permission) {
    return true;
  }

  return subject.permissions.includes(permission) || subject.permissions.includes("*");
}

export function filterNavigation(items: NavigationItem[], subject: RbacSubject): NavigationItem[] {
  return items
    .map((item) => {
      if (item.children?.length) {
        const children = filterNavigation(item.children, subject);
        if (children.length === 0) {
          return item.permission && hasPermission(subject, item.permission) ? { ...item, children } : null;
        }

        return { ...item, children };
      }

      return hasPermission(subject, item.permission) ? item : null;
    })
    .filter((item): item is NavigationItem => item !== null);
}
