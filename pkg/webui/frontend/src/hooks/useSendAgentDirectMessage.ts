import { useMutation } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toast } from "@/lib/notify";

export interface SendAgentDirectMessageRequest {
  content: string;
  attachment_ids?: string[];
  reply_to_message_id?: string;
  request_id: string;
}

export interface SendAgentDirectMessageResponse {
  message_id: string;
  agent_id: string;
  content: string;
  created_at: string;
  target: string;
}

export function useSendAgentDirectMessage() {
  return useMutation({
    mutationFn: async (req: SendAgentDirectMessageRequest & { agentId: string }) => {
      const { agentId, ...body } = req;

      if (!agentId) {
        throw new Error("Agent ID is required");
      }

      if (!body.content || !body.content.trim()) {
        throw new Error("Message content cannot be empty");
      }

      try {
        // Call the gRPC SendMessage RPC through daemon client
        // This uses the existing collaboration protocol, not a new HTTP wrapper
        const response = await api.post<SendAgentDirectMessageResponse>(
          `/daemon/agents/${agentId}/message`,
          body
        );
        return response;
      } catch (error) {
        if (error instanceof Error) {
          throw error;
        }
        throw new Error("Failed to send direct message");
      }
    },
    onError: (error) => {
      const message = error instanceof Error ? error.message : "Unknown error";
      toast.error(`Failed to send message: ${message}`);
    },
    onSuccess: (data) => {
      toast.success(`Message sent to ${data.agent_id}`);
    },
  });
}


