export interface Notification {
    id: number;
    initiatorId: number;
    initiatorName: string;
    receiverId: number;
    ticketUrl: string;
    createdTs: number;
    isRead: boolean;
}
