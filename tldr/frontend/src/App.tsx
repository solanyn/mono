import { Routes, Route, Navigate } from "react-router-dom";
import NewsLayout from "./routes/NewsLayout";
import NewsPage from "./routes/NewsPage";

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/news" replace />} />
      <Route path="/news" element={<NewsLayout />}>
        <Route index element={<div>Select a date</div>} />
        <Route path=":date" element={<NewsPage />} />
      </Route>
    </Routes>
  );
}
