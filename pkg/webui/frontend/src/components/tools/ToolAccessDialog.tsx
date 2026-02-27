import { useState, useEffect, useRef, useCallback } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { t } from '@/lib/i18n';
import { toast } from 'sonner';
import {
  useUpdateAccess,
  useGenerateOTP,
  type ToolSession,
} from '@/hooks/useToolSessions';
import { Copy, RefreshCw } from 'lucide-react';

const ACCESS_RECORDS_KEY = 'nekobot_tool_access_records';

interface AccessRecord {
  mode: string;
  url: string;
  password: string;
  updated_at: number;
}

function loadAccessRecords(): Record<string, AccessRecord> {
  try {
    const raw = localStorage.getItem(ACCESS_RECORDS_KEY) || '{}';
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function saveAccessRecord(
  sessionId: string,
  mode: string,
  url: string,
  password: string,
) {
  const records = loadAccessRecords();
  records[sessionId] = {
    mode,
    url,
    password,
    updated_at: Date.now(),
  };
  try {
    localStorage.setItem(ACCESS_RECORDS_KEY, JSON.stringify(records));
  } catch {
    /* ignore */
  }
}

export function getAccessRecord(sessionId: string): AccessRecord | null {
  const records = loadAccessRecords();
  const rec = records[sessionId];
  if (!rec || !rec.url || !rec.password) return null;
  return rec;
}

interface ToolAccessDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  session: ToolSession | null;
  initialUrl?: string;
  initialPassword?: string;
}

export default function ToolAccessDialog({
  open,
  onOpenChange,
  session,
  initialUrl,
  initialPassword,
}: ToolAccessDialogProps) {
  const [accessUrl, setAccessUrl] = useState('');
  const [password, setPassword] = useState('');
  const [otpCode, setOtpCode] = useState('');
  const [otpExpiresAt, setOtpExpiresAt] = useState(0);
  const [otpTtl, setOtpTtl] = useState(180_000);
  const [otpCountdown, setOtpCountdown] = useState('');
  const [otpProgress, setOtpProgress] = useState(0);
  const otpTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const accessMutation = useUpdateAccess();
  const otpMutation = useGenerateOTP();

  /* ---- Load or fetch access credentials ---- */
  const loadAccess = useCallback(
    async (forceRefresh: boolean) => {
      if (!session) return;

      /* Try cached record first */
      if (!forceRefresh) {
        if (initialUrl && initialPassword) {
          setAccessUrl(initialUrl);
          setPassword(initialPassword);
          saveAccessRecord(session.id, session.access_mode, initialUrl, initialPassword);
          return;
        }
        const cached = getAccessRecord(session.id);
        if (cached) {
          setAccessUrl(cached.url);
          setPassword(cached.password);
          return;
        }
      }

      /* Fetch from API */
      const mode = (session.access_mode || '').trim();
      if (!mode || mode === 'none') {
        toast.warning(t('externalAccessDisabled'));
        return;
      }

      try {
        const result = await accessMutation.mutateAsync({
          id: session.id,
          mode,
        });
        const url = result.access_url || '';
        const pw = result.access_password || '';
        if (!url || !pw) {
          toast.error(t('accessNotAvailable'));
          return;
        }
        setAccessUrl(url);
        setPassword(pw);
        saveAccessRecord(session.id, result.access_mode || mode, url, pw);
      } catch {
        /* error handled by mutation */
      }
    },
    [session, initialUrl, initialPassword, accessMutation],
  );

  /* ---- Fetch OTP ---- */
  const refreshOtp = useCallback(async () => {
    if (!session) return;
    try {
      const result = await otpMutation.mutateAsync(session.id);
      const code = (result.otp_code || '').trim();
      const expiresAt = (result.expires_at || 0) * 1000;
      const ttl = Math.max(1000, (result.ttl_seconds || 180) * 1000);
      if (!code || !expiresAt) {
        toast.error(t('otpUnavailable'));
        return;
      }
      setOtpCode(code);
      setOtpExpiresAt(expiresAt);
      setOtpTtl(ttl);
    } catch {
      setOtpCode('');
      setOtpExpiresAt(0);
    }
  }, [session, otpMutation]);

  /* ---- OTP countdown timer ---- */
  useEffect(() => {
    if (otpTimerRef.current) {
      clearInterval(otpTimerRef.current);
      otpTimerRef.current = null;
    }
    if (!otpExpiresAt) {
      setOtpCountdown('-');
      setOtpProgress(0);
      return;
    }
    function tick() {
      const leftMs = Math.max(0, otpExpiresAt - Date.now());
      if (leftMs <= 0) {
        setOtpCountdown(t('expired'));
        setOtpProgress(0);
        if (otpTimerRef.current) {
          clearInterval(otpTimerRef.current);
          otpTimerRef.current = null;
        }
        return;
      }
      const progress = leftMs / otpTtl;
      setOtpProgress(progress);
      setOtpCountdown(Math.ceil(leftMs / 1000) + 's');
    }
    tick();
    otpTimerRef.current = setInterval(tick, 250);
    return () => {
      if (otpTimerRef.current) {
        clearInterval(otpTimerRef.current);
        otpTimerRef.current = null;
      }
    };
  }, [otpExpiresAt, otpTtl]);

  /* ---- Init on open ---- */
  useEffect(() => {
    if (!open) {
      setOtpCode('');
      setOtpExpiresAt(0);
      return;
    }
    loadAccess(false);
    if (session) refreshOtp();
  }, [open]); // eslint-disable-line react-hooks/exhaustive-deps

  function copyToClipboard(text: string, label: string) {
    navigator.clipboard
      .writeText(text)
      .then(() => toast.success(t('copied') + ': ' + label))
      .catch(() => {});
  }

  const sessionHint = session
    ? (session.title || session.tool || session.id).trim() +
      ' \u00b7 ' +
      session.id.slice(0, 8)
    : '';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('toolAccessDialogTitle')}</DialogTitle>
          <DialogDescription>
            {sessionHint}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Access URL */}
          <div className="space-y-2">
            <Label>{t('accessUrl')}</Label>
            <div className="flex gap-2">
              <Input value={accessUrl} readOnly className="font-mono text-xs" />
              <Button
                variant="outline"
                size="icon"
                onClick={() => copyToClipboard(accessUrl, t('accessUrl'))}
                title={t('copyUrl')}
                aria-label={t('copyUrl')}
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Permanent password */}
          <div className="space-y-2">
            <Label>{t('permanentPassword')}</Label>
            <div className="flex gap-2">
              <Input value={password} readOnly className="font-mono text-xs" />
              <Button
                variant="outline"
                size="icon"
                onClick={() =>
                  copyToClipboard(password, t('permanentPassword'))
                }
                title={t('copyPassword')}
                aria-label={t('copyPassword')}
              >
                <Copy className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Refresh access */}
          <Button
            variant="outline"
            size="sm"
            onClick={() => loadAccess(true)}
            disabled={accessMutation.isPending}
          >
            <RefreshCw className="h-3.5 w-3.5 mr-1.5" />
            {t('refreshAccess')}
          </Button>

          {/* OTP section */}
          <div className="space-y-2 pt-2 border-t">
            <Label>{t('otpPassword')}</Label>
            <div className="flex gap-2 items-center">
              <Input
                value={otpCode}
                readOnly
                className="font-mono text-xs flex-1"
              />
              <Button
                variant="outline"
                size="icon"
                onClick={() => copyToClipboard(otpCode, t('otpPassword'))}
                title={t('copyOtp')}
                disabled={!otpCode}
                aria-label={t('copyOtp')}
              >
                <Copy className="h-4 w-4" />
              </Button>
              <Button
                variant="outline"
                size="icon"
                onClick={refreshOtp}
                disabled={otpMutation.isPending}
                title={t('refreshOtp')}
                aria-label={t('refreshOtp')}
              >
                <RefreshCw className="h-4 w-4" />
              </Button>
            </div>

            {/* OTP countdown ring */}
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <div className="relative w-6 h-6">
                <svg viewBox="0 0 24 24" aria-hidden="true" className="w-6 h-6 -rotate-90">
                  <circle
                    cx="12"
                    cy="12"
                    r="10"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    className="opacity-20"
                  />
                  <circle
                    cx="12"
                    cy="12"
                    r="10"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeDasharray={`${otpProgress * 62.83} 62.83`}
                    className="text-primary transition-all duration-200"
                  />
                </svg>
              </div>
              <span>{otpCountdown}</span>
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
