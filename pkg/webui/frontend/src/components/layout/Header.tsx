import { cn } from '@/lib/utils';

interface HeaderProps {
  title?: string;
  description?: string;
  className?: string;
}

export default function Header({ title, description, className }: HeaderProps) {
  return (
    <header className={cn('mb-5 sm:mb-6', className)}>
      {title && (
        <div className="max-w-3xl">
          <div className="mb-3 h-px w-16 rounded-full bg-gradient-to-r from-[hsl(var(--brand-400))/0.8] via-[hsl(var(--brand-300))/0.45] to-transparent" />
          <h1 className="text-[1.8rem] font-semibold text-foreground tracking-[-0.03em] sm:text-[2.1rem]">
            {title}
          </h1>
          {description && (
            <p className="mt-2 text-sm leading-6 text-muted-foreground sm:text-[15px]">{description}</p>
          )}
        </div>
      )}
    </header>
  );
}
