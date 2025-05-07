import {
  Box,
  Drawer,
  Link,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Toolbar,
  Typography,
  CircularProgress,
  Alert,
} from "@mui/material";
import { Outlet, useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import protobuf from "protobufjs";
import { ListNewsSummaries } from "../proto/news_pb"; // Imported generated types
import { formatDate } from "../utils/date";

const drawerWidth = 240;
const API_URL = import.meta.env.VITE_API_URL as string;

const fetchNewsSummaries = async (): Promise<ListNewsSummariesResponse> => {
  const res = await fetch(`${API_URL}/api/news`);
  if (!res.ok) throw new Error("Failed to fetch news summaries");
  const json = await res.json();

  const root = await protobuf.load("/proto/news.proto");
  const ListNewsSummariesResponseType = root.lookupType(
    "tldr.news.v1.ListNewsSummariesResponse",
  );

  const errMsg = ListNewsSummariesResponseType.verify(json);
  if (errMsg) throw new Error(`Invalid response: ${errMsg}`);

  return ListNewsSummariesResponseType.fromObject(json);
};

export default function NewsLayout() {
  const navigate = useNavigate();
  const { date: selectedDate } = useParams<{ date: string }>();

  const {
    data: summaries,
    isLoading,
    isError,
  } = useQuery<ListNewsSummaries>({
    queryKey: ["newsSummaries"],
    queryFn: fetchNewsSummaries,
  });

  return (
    <Box sx={{ display: "flex", minHeight: "100vh", flexDirection: "column" }}>
      <Box sx={{ display: "flex", flex: 1 }}>
        <Drawer
          variant="permanent"
          sx={{
            width: drawerWidth,
            flexShrink: 0,
            "& .MuiDrawer-paper": {
              width: drawerWidth,
              boxSizing: "border-box",
            },
          }}
        >
          <Toolbar />
          <Box sx={{ overflow: "auto", p: 2 }}>
            <Typography variant="h6">tl;dr news</Typography>
            {isLoading ? (
              <CircularProgress />
            ) : isError ? (
              <Alert severity="error">Failed to load summaries</Alert>
            ) : (
              <List>
                {summaries?.summaries.map((summary) => (
                  <ListItem key={summary.date} disablePadding>
                    <ListItemButton
                      selected={selectedDate === summary.date}
                      onClick={() => navigate(`/news/${summary.date}`)}
                    >
                      <ListItemText primary={formatDate(summary.date)} />
                    </ListItemButton>
                  </ListItem>
                ))}
              </List>
            )}
          </Box>
        </Drawer>
        <Box
          component="main"
          sx={{ flexGrow: 1, p: 3, display: "flex", flexDirection: "column" }}
        >
          <Toolbar />
          <Box sx={{ flex: 1 }}>
            <Outlet />
          </Box>
          <Box component="footer" sx={{ mt: 4, textAlign: "center", py: 2 }}>
            <Typography variant="body2" color="text.secondary">
              Made with{" "}
              <span role="img" aria-label="heart">
                💚
              </span>{" "}
              by{" "}
              <Link
                href="https://solanyn.dev"
                target="_blank"
                rel="noopener noreferrer"
              >
                solanyn.dev
              </Link>
            </Typography>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
