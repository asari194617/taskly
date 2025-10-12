import * as React from "react";
import { cn } from "../../utils/cn";

export function Empty({
  className,
  children,
}: React.PropsWithChildren<{ className?: string }>) {
  return <div className={cn("w-full rounded-lg", className)}>{children}</div>;
}

export function EmptyHeader({
  className,
  children,
}: React.PropsWithChildren<{ className?: string }>) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center text-center gap-2 p-8",
        className
      )}
    >
      {children}
    </div>
  );
}

export function EmptyMedia({
  className,
  variant = "default",
  children,
}: React.PropsWithChildren<{
  className?: string;
  variant?: "default" | "icon";
}>) {
  return (
    <div
      className={cn(
        "flex items-center justify-center overflow-hidden rounded-full",
        variant === "icon"
          ? "h-12 w-12 border bg-muted text-muted-foreground"
          : "h-16 w-16",
        className
      )}
    >
      {children}
    </div>
  );
}

export function EmptyTitle({
  className,
  children,
}: React.PropsWithChildren<{ className?: string }>) {
  return (
    <div className={cn("text-base font-medium text-foreground", className)}>
      {children}
    </div>
  );
}

export function EmptyDescription({
  className,
  children,
}: React.PropsWithChildren<{ className?: string }>) {
  return (
    <p className={cn("text-sm text-muted-foreground", className)}>{children}</p>
  );
}

export function EmptyContent({
  className,
  children,
}: React.PropsWithChildren<{ className?: string }>) {
  return (
    <div
      className={cn("flex items-center justify-center gap-2 p-4", className)}
    >
      {children}
    </div>
  );
}
