import * as React from 'react';
import { cn } from '@/lib/utils';
import { Label } from './label';
import { Input } from './input';

export interface FormFieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
  required?: boolean;
  id: string;
}

const FormField = React.forwardRef<HTMLInputElement, FormFieldProps>(
  ({ className, label, error, helperText, required, id, ...props }, ref) => {
    return (
      <div className={cn('space-y-1.5', className)}>
        {label && (
          <Label htmlFor={id} className={cn(error && 'text-destructive')}>
            {label}
            {required && <span className="ml-1 text-destructive">*</span>}
          </Label>
        )}
        <Input
          id={id}
          ref={ref}
          aria-invalid={error ? 'true' : 'false'}
          aria-describedby={error ? `${id}-error` : helperText ? `${id}-helper` : undefined}
          className={cn(error && 'border-destructive focus-visible:ring-destructive')}
          {...props}
        />
        {error && (
          <p id={`${id}-error`} className="text-sm text-destructive" role="alert">
            {error}
          </p>
        )}
        {helperText && !error && (
          <p id={`${id}-helper`} className="text-sm text-muted-foreground">
            {helperText}
          </p>
        )}
      </div>
    );
  }
);

FormField.displayName = 'FormField';

export { FormField };
