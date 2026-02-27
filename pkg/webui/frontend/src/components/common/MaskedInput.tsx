import { useState } from 'react';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Eye, EyeOff } from 'lucide-react';
import { cn } from '@/lib/utils';

interface MaskedInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  /** When true, shows the toggle button even if the input is empty (indicates a value is already set server-side). */
  isSet?: boolean;
}

export function MaskedInput({ isSet, className, value, ...props }: MaskedInputProps) {
  const [visible, setVisible] = useState(false);

  return (
    <div className="relative">
      <Input
        type={visible ? 'text' : 'password'}
        className={cn('pr-10', className)}
        value={value}
        {...props}
      />
      {(isSet || (typeof value === 'string' && value.length > 0)) && (
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 text-muted-foreground hover:text-foreground"
          onClick={() => setVisible((v) => !v)}
          tabIndex={-1}
          aria-label={visible ? 'Hide password' : 'Show password'}
        >
          {visible ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
        </Button>
      )}
    </div>
  );
}
