import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Plus, Trash2 } from 'lucide-react';
import { t } from '@/lib/i18n';
import { cn } from '@/lib/utils';

interface KeyValueEditorProps {
  value: Record<string, string> | null;
  onChange: (value: Record<string, string>) => void;
  keyPlaceholder?: string;
  valuePlaceholder?: string;
  className?: string;
}

export function KeyValueEditor({
  value,
  onChange,
  keyPlaceholder,
  valuePlaceholder,
  className,
}: KeyValueEditorProps) {
  const entries = value ? Object.entries(value) : [];

  const updateEntry = (index: number, key: string, val: string) => {
    const updated = [...entries];
    updated[index] = [key, val];
    onChange(Object.fromEntries(updated));
  };

  const addEntry = () => {
    onChange({ ...value, '': '' });
  };

  const removeEntry = (index: number) => {
    const filtered = entries.filter((_, i) => i !== index);
    onChange(Object.fromEntries(filtered));
  };

  return (
    <div className={cn('space-y-2', className)}>
      {entries.map(([key, val], index) => (
        <div key={index} className="flex gap-2 items-center">
          <Input
            type="text"
            value={key}
            onChange={(e) => updateEntry(index, e.target.value, val)}
            placeholder={keyPlaceholder ?? t('headerName')}
            className="flex-1"
          />
          <Input
            type="text"
            value={val}
            onChange={(e) => updateEntry(index, key, e.target.value)}
            placeholder={valuePlaceholder ?? t('headerValue')}
            className="flex-1"
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="shrink-0 text-destructive hover:text-destructive"
            onClick={() => removeEntry(index)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" onClick={addEntry}>
        <Plus className="h-4 w-4 mr-1.5" />
        {t('add')}
      </Button>
    </div>
  );
}
