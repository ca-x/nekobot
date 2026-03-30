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
          <h1 className="text-[1.75rem] font-semibold text-foreground tracking-tight sm:text-[2rem]">
            {title}
          </h1>
          {description && (
            <p className="mt-1.5 text-sm leading-6 text-muted-foreground sm:text-[15px]">{description}</p>
          )}
        </div>
      )}
    </header>
  );
}
