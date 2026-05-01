import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { toast } from "@/lib/notify";
import { MessageSquare, Send } from "lucide-react";

export interface DirectAgentMessageDialogProps {
  agentId: string;
  agentName?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  onSendMessage: (agentId: string, content: string, requestId: string) => Promise<void>;
}

export default function DirectAgentMessageDialog({
  agentId,
  agentName,
  open = false,
  onOpenChange,
  onSendMessage,
}: DirectAgentMessageDialogProps) {
  const [content, setContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const messageInputId = `direct-agent-message-${agentId || "agent"}`;

  const handleSend = async () => {
    if (!content.trim()) {
      toast.error("Message cannot be empty");
      return;
    }

    setIsLoading(true);
    try {
      const requestId = `msg-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
      await onSendMessage(agentId, content, requestId);
      setContent("");
      onOpenChange?.(false);
    } catch (error) {
      toast.error(`Failed to send message: ${error instanceof Error ? error.message : "Unknown error"}`);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <MessageSquare className="h-5 w-5" />
            Direct Message to {agentName || agentId}
          </DialogTitle>
          <DialogDescription>
            Send a direct message to this agent. The message will be delivered via the collaboration channel.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <Label htmlFor={messageInputId}>Message</Label>
          <Textarea
            id={messageInputId}
            aria-label={`Message to ${agentName || agentId}`}
            placeholder="Type your message here..."
            value={content}
            onChange={(e) => setContent(e.target.value)}
            disabled={isLoading}
            className="min-h-[120px]"
          />
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange?.(false)}
            disabled={isLoading}
          >
            Cancel
          </Button>
          <Button
            onClick={handleSend}
            disabled={isLoading || !content.trim()}
            className="gap-2"
          >
            <Send className="h-4 w-4" />
            {isLoading ? "Sending..." : "Send"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
