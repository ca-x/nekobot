import { Suspense, lazy, type ReactNode } from 'react';

type AuthGradientVariant = 'login' | 'init';

interface AuthGradientShellProps {
  children: ReactNode;
  variant?: AuthGradientVariant;
}

const ShaderGradientBackdrop = lazy(() => import('./ShaderGradientBackdrop'));

export default function AuthGradientShell({
  children,
  variant = 'login',
}: AuthGradientShellProps) {
  return (
    <div className="relative isolate min-h-screen overflow-hidden bg-background">
      <div className="pointer-events-none absolute inset-0">
        <Suspense fallback={null}>
          <ShaderGradientBackdrop variant={variant} className="h-full w-full" />
        </Suspense>
      </div>

      <div
        className="pointer-events-none absolute inset-0"
        aria-hidden="true"
      >
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,hsl(var(--brand-200))/0.34,transparent_34%),radial-gradient(circle_at_80%_18%,hsl(var(--brand-400))/0.2,transparent_24%),linear-gradient(180deg,hsl(var(--background))/0.2,transparent_35%,hsl(var(--background))/0.6_100%)] dark:bg-[radial-gradient(circle_at_top,hsl(var(--brand-500))/0.24,transparent_28%),radial-gradient(circle_at_80%_18%,hsl(var(--brand-400))/0.15,transparent_22%),linear-gradient(180deg,hsl(var(--background))/0.18,transparent_35%,hsl(var(--background))/0.72_100%)]" />
        <div className="absolute inset-0 backdrop-blur-[72px]" />
      </div>

      <div className="relative z-10">{children}</div>
    </div>
  );
}
