import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "@/lib/notify";
import { t } from "@/lib/i18n";
import {
  Power,
  RotateCw,
  RotateCcw,
  Zap,
  MessageSquare,
  ChevronDown,
} from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu";

export interface AgentLifecycleControlsProps {
  agentId: string;
  agentName?: string;
  isSupported?: boolean;
  isDisabled?: boolean;
  operationState?: "unsupported" | "requested" | "queued" | "running" | "completed" | "failed" | "timed_out";
  lastOperationTime?: string;
  onTerminate?: (agentId: string, requestId: string) => Promise<void>;
  onRestart?: (agentId: string, requestId: string) => Promise<void>;
  onResetSession?: (agentId: string, requestId: string) => Promise<void>;
  onFullReset?: (agentId: string, requestId: string) => Promise<void>;
  onDirectMessage?: (agentId: string) => void;
}

export default function AgentLifecycleControls({
  agentId,
  agentName,
  isSupported = false,
  isDisabled = false,
  operationState = "unsupported",
  lastOperationTime,
  onTerminate,
  onRestart,
  onResetSession,
  onFullReset,
  onDirectMessage,
}: AgentLifecycleControlsProps) {
  const [confirmAction, setConfirmAction] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);

  const isPending = ["requested", "queued", "running"].includes(operationState);
  const isFailed = operationState === "failed";
  const isCompleted = operationState === "completed";

  const getButtonState = () => {
    if (isDisabled || !agentId) return "disabled";
    if (isPending) return "pending";
    if (isFailed) return "failed";
    if (!isSupported) return "unsupported";
    return "enabled";
  };

  const buttonState = getButtonState();
  const canExecute = buttonState === "enabled" || buttonState === "failed";

  const handleAction = async (action: string) => {
    if (!canExecute) return;

    setIsLoading(true);
    try {
      const requestId = `${action}-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;

      switch (action) {
        case "terminate":
          await onTerminate?.(agentId, requestId);
          toast.success(`Terminate request sent for ${agentName || agentId}`);
          break;
        case "restart":
          await onRestart?.(agentId, requestId);
          toast.success(`Restart request sent for ${agentName || agentId}`);
          break;
        case "reset_session":
          await onResetSession?.(agentId, requestId);
          toast.success(`Reset session request sent for ${agentName || agentId}`);
          break;
        case "full_reset":
          await onFullReset?.(agentId, requestId);
          toast.success(`Full reset request sent for ${agentName || agentId}`);
          break;
      }

      setConfirmAction(null);
    } catch (error) {
      toast.error(`Failed to execute ${action}: ${error instanceof Error ? error.message : "Unknown error"}`);
    } finally {
      setIsLoading(false);
    }
  };

  const getStateLabel = () => {
    switch (buttonState) {
      case "disabled":
        return "Agent unavailable";
      case "pending":
        return `Operation ${operationState}...`;
      case "unsupported":
        return "Not supported";
      case "failed":
        return "Last operation failed";
      case "enabled":
        return isCompleted ? "Ready" : "Ready";
      default:
        return "";
    }
  };

  const getStateColor = () => {
    switch (buttonState) {
      case "pending":
        return "text-yellow-600";
      case "failed":
        return "text-red-600";
      case "unsupported":
        return "text-gray-500";
      default:
        return "text-gray-600";
    }
  };

  return (
    <div className="flex items-center gap-2">
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            disabled={!canExecute}
            className="gap-1"
          >
            <Zap className="h-4 w-4" />
            <span>Control</span>
            <ChevronDown className="h-3 w-3" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-48">
          <DropdownMenuItem
            onClick={() => onDirectMessage?.(agentId)}
            disabled={!agentId}
          >
            <MessageSquare className="h-4 w-4 mr-2" />
            <span>Direct Message</span>
          </DropdownMenuItem>

          <DropdownMenuSeparator />

          <DropdownMenuItem
            onClick={() => setConfirmAction("terminate")}
            disabled={!canExecute}
          >
            <Power className="h-4 w-4 mr-2" />
            <span>Terminate</span>
          </DropdownMenuItem>

          <DropdownMenuItem
            onClick={() => setConfirmAction("restart")}
            disabled={!canExecute}
          >
            <RotateCw className="h-4 w-4 mr-2" />
            <span>Restart</span>
          </DropdownMenuItem>

          <DropdownMenuItem
            onClick={() => setConfirmAction("reset_session")}
            disabled={!canExecute}
          >
            <RotateCcw className="h-4 w-4 mr-2" />
            <span>Reset Session & Restart</span>
          </DropdownMenuItem>

          <DropdownMenuItem
            onClick={() => setConfirmAction("full_reset")}
            disabled={!canExecute}
          >
            <Zap className="h-4 w-4 mr-2" />
            <span>Full Reset & Restart</span>
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>

      {buttonState !== "enabled" && (
        <span className={`text-xs ${getStateColor()}`}>
          {getStateLabel()}
        </span>
      )}

      {/* Confirmation Dialogs */}
      <AlertDialog open={confirmAction === "terminate"} onOpenChange={(open) => !open && setConfirmAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Terminate Agent</AlertDialogTitle>
            <AlertDialogDescription>
              Stop the current process for {agentName || agentId}. Collaboration history and configuration will be preserved.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleAction("terminate")}
              disabled={isLoading}
            >
              {isLoading ? "Sending..." : "Terminate"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={confirmAction === "restart"} onOpenChange={(open) => !open && setConfirmAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Restart Agent</AlertDialogTitle>
            <AlertDialogDescription>
              Restart {agentName || agentId} without clearing session context. All collaboration history and settings will be preserved.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleAction("restart")}
              disabled={isLoading}
            >
              {isLoading ? "Sending..." : "Restart"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={confirmAction === "reset_session"} onOpenChange={(open) => !open && setConfirmAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Reset Session & Restart</AlertDialogTitle>
            <AlertDialogDescription>
              Clear the current chat context for {agentName || agentId}, then restart. Task history, runs, and configuration will be preserved.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleAction("reset_session")}
              disabled={isLoading}
            >
              {isLoading ? "Sending..." : "Reset & Restart"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <AlertDialog open={confirmAction === "full_reset"} onOpenChange={(open) => !open && setConfirmAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Full Reset & Restart</AlertDialogTitle>
            <AlertDialogDescription className="space-y-2">
              <p>
                Clear runtime-local state for {agentName || agentId}, then restart.
              </p>
              <p className="text-sm font-semibold">
                ✓ Preserved: collaboration messages, tasks, runs, event log, environment, permissions, audit trail
              </p>
              <p className="text-sm font-semibold">
                ✗ Cleared: runtime session cache, adapter-local state
              </p>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => handleAction("full_reset")}
              disabled={isLoading}
              className="bg-destructive"
            >
              {isLoading ? "Sending..." : "Full Reset & Restart"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
