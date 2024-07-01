import React from "react";
import { Link } from "react-router-dom";

const Error404 = () => {
  return (
    <div className="flex flex-col items-center justify-center h-screen bg-gray-100 text-gray-800 text-center">
      <h1 className="text-6xl font-bold mb-4">404 - Page Not Found</h1>
      <p className="text-2xl mb-6">
        Sorry, the page you are looking for does not exist.
      </p>
      <Link to="/" className="text-lg text-blue-500 hover:underline">
        Go to Home
      </Link>
    </div>
  );
};

export default Error404;
