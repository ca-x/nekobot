import * as React from 'react';
import { Slot } from '@radix-ui/react-slot';

import { cn } from '@/lib/utils';

interface DropdownMenuContextValue {
  open: boolean;
  setOpen: (open: boolean) => void;
}

const DropdownMenuContext = React.createContext<DropdownMenuContextValue | null>(null);

function useDropdownMenu() {
  const ctx = React.useContext(DropdownMenuContext);
  if (!ctx) {
    throw new Error('DropdownMenu components must be used inside DropdownMenu');
  }
  return ctx;
}

function DropdownMenu({ children }: { children: React.ReactNode }) {
  const [open, setOpen] = React.useState(false);
  const rootRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    if (!open) return;
    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setOpen(false);
      }
    };
    document.addEventListener('pointerdown', handlePointerDown);
    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown);
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [open]);

  return (
    <DropdownMenuContext.Provider value={{ open, setOpen }}>
      <div ref={rootRef} className="relative inline-flex">
        {children}
      </div>
    </DropdownMenuContext.Provider>
  );
}

function DropdownMenuTrigger({
  asChild,
  children,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { asChild?: boolean }) {
  const { open, setOpen } = useDropdownMenu();
  const Comp = asChild ? Slot : 'button';
  return (
    <Comp
      {...props}
      aria-expanded={open}
      aria-haspopup="menu"
      onClick={(event: React.MouseEvent<HTMLElement>) => {
        (props.onClick as React.MouseEventHandler<HTMLElement> | undefined)?.(event);
        if (!event.defaultPrevented) {
          setOpen(!open);
        }
      }}
    >
      {children}
    </Comp>
  );
}

function DropdownMenuContent({
  className,
  align = 'start',
  children,
}: React.HTMLAttributes<HTMLDivElement> & { align?: 'start' | 'end' | 'center' }) {
  const { open } = useDropdownMenu();
  if (!open) return null;
  return (
    <div
      role="menu"
      className={cn(
        'absolute top-full z-50 mt-2 min-w-40 rounded-xl border border-border/70 bg-popover/96 p-1 text-popover-foreground shadow-lg backdrop-blur-xl',
        align === 'end' && 'right-0',
        align === 'center' && 'left-1/2 -translate-x-1/2',
        align === 'start' && 'left-0',
        className,
      )}
    >
      {children}
    </div>
  );
}

function DropdownMenuItem({
  className,
  disabled,
  onClick,
  children,
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const { setOpen } = useDropdownMenu();
  return (
    <button
      type="button"
      role="menuitem"
      disabled={disabled}
      onClick={(event) => {
        onClick?.(event);
        if (!event.defaultPrevented) {
          setOpen(false);
        }
      }}
      className={cn(
        'flex w-full items-center rounded-lg px-2.5 py-2 text-left text-sm outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground disabled:pointer-events-none disabled:opacity-50',
        className,
      )}
    >
      {children}
    </button>
  );
}

function DropdownMenuSeparator({ className }: React.HTMLAttributes<HTMLDivElement>) {
  return <div role="separator" className={cn('-mx-1 my-1 h-px bg-muted', className)} />;
}

export {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
};
