interface HeaderProps {
  title?: string;
  description?: string;
}

export default function Header({ title, description }: HeaderProps) {
  return (
    <header className="mb-6">
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
