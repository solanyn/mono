import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { Box, CircularProgress, Typography, Alert } from "@mui/material";
import ReactMarkdown from "react-markdown";
import protobuf from "protobufjs";
import { GetNewsSummaryResponse } from "../proto/news_pb";
import { formatDate } from "../utils/date";

const API_URL = import.meta.env.VITE_API_URL as string;

const fetchAndValidateNews = async (date: string): Promise<string> => {
  const res = await fetch(`${API_URL}/api/news/${date}`);
  if (!res.ok) throw new Error("Failed to fetch content");

  const json = await res.json();

  const root = await protobuf.load("/proto/news.proto");
  const GetNewsSummaryResponseType = root.lookupType(
    "tldr.news.v1.GetNewsSummaryResponse",
  );

  const errMsg = GetNewsSummaryResponseType.verify(json);
  if (errMsg) throw new Error(`Invalid response: ${errMsg}`);

  const message = GetNewsSummaryResponseType.fromObject(json);
  return message.content;
};

export default function NewsPage() {
  const { date } = useParams<{ date: string }>();

  const {
    data: markdown,
    isLoading,
    isError,
  } = useQuery<string>({
    queryKey: ["newsMarkdown", date],
    queryFn: () => fetchAndValidateNews(date!),
    enabled: !!date,
  });

  return (
    <Box>
      <Typography variant="h4" gutterBottom>
        tl;dr on {formatDate(date!)}
      </Typography>
      {isLoading ? (
        <CircularProgress />
      ) : isError ? (
        <Alert severity="error">Failed to load content</Alert>
      ) : (
        <Box sx={{ mt: 2 }}>
          <ReactMarkdown>{markdown!}</ReactMarkdown>
        </Box>
      )}
    </Box>
  );
}
