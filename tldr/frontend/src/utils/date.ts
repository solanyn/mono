import { format, parseISO } from "date-fns";

export const formatDate = (isoDate: string): string => {
  const date = parseISO(isoDate);
  return format(date, "MMMM do, yyyy");
};
