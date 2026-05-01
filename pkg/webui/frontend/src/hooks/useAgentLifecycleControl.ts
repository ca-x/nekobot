import { useMutation } from "@tanstack/react-query";
import { api } from "@/api/client";
import { toast } from "@/lib/notify";

export interface ControlAgentRequest {
  action: "terminate" | "restart" | "restart_reset_session" | "restart_full_reset";
  reason?: string;
  computer_id?: string;
  runtime_profile_id?: string;
  request_id: string;
}

export interface ControlAgentResponse {
  operation_id: string;
  agent_id: string;
  action: string;
  state: "unsupported" | "requested" | "queued" | "running" | "completed" | "failed" | "timed_out";
  accepted: boolean;
  created_at: string;
  updated_at?: string;
  error_message?: string;
}

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
}

export function useControlAgent() {
  return useMutation({
    mutationFn: async (req: ControlAgentRequest & { agentId: string }) => {
      const { agentId, ...body } = req;
      const response = await api.post<ControlAgentResponse>(
        `/api/daemon/agents/${agentId}/control`,
        body
      );
      return response;
    },
    onError: (error) => {
      toast.error(`Failed to control agent: ${error instanceof Error ? error.message : "Unknown error"}`);
    },
  });
}

export function useSendAgentDirectMessage() {
  return useMutation({
    mutationFn: async (req: SendAgentDirectMessageRequest & { agentId: string }) => {
      const { agentId, ...body } = req;
      const response = await api.post<SendAgentDirectMessageResponse>(
        `/api/daemon/agents/${agentId}/message`,
        body
      );
      return response;
    },
    onError: (error) => {
      toast.error(`Failed to send message: ${error instanceof Error ? error.message : "Unknown error"}`);
    },
  });
}
