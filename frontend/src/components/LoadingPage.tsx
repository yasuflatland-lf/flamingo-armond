import React from "react";
import { GiFlamingo } from "react-icons/gi";

function LoadingPage() {
  return (
    <div
      className="bg-pink-700 flex flex-col items-center justify-center h-screen bg-gray-100"
      data-testid="loading-container"
    >
      <GiFlamingo
        className="text-white text-8xl animate-flip"
        data-testid="flamingo-icon"
      />
      <p className="text-lg text-white mt-4">Loading...</p>
    </div>
  );
}

export default LoadingPage;
