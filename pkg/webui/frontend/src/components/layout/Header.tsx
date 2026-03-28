import { cn } from '@/lib/utils';

interface HeaderProps {
  title?: string;
  description?: string;
  className?: string;
}

export default function Header({ title, description, className }: HeaderProps) {
  return (
    <header className={cn('mb-6', className)}>
      {title && (
        <div>
          <h1 className="text-2xl font-semibold text-foreground tracking-tight">
            {title}
          </h1>
          {description && (
            <p className="text-sm text-muted-foreground mt-1">{description}</p>
          )}
        </div>
      )}
    </header>
  );
}
