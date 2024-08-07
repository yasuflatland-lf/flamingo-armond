import React, { useState, KeyboardEvent } from "react";
import { IoIosSearch, IoIosSettings } from "react-icons/io";
import { Link } from "react-router-dom";

function TopMenu() {
  const [searchTerm, setSearchTerm] = useState("");

  const handleKeyDown = async (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      // Call the REST API
      try {
        const response = await fetch("https://api.example.com/search", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({ query: searchTerm }),
        });

        if (response.ok) {
          const data = await response.json();
          console.log(data); // Handle the response data
        } else {
          console.error("API call failed");
        }
      } catch (error) {
        console.error("Error:", error);
      }
    }
  };

  return (
    <div className="bg-white border border-gray-200">
      <div className="fixed top-0 w-full flex justify-end py-2.5 pr-4 z-50">
        <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
          <Link to="/settings">
            <IoIosSettings
              className="text-gray-700 text-2xl"
              data-testid="settings-icon"
            />
          </Link>
        </button>
      </div>
      <div className="pt-12 px-4 pb-6">
        <div className="relative">
          <input
            type="text"
            className="w-full pl-10 pr-4 py-2 border border-gray-500 rounded-lg focus:outline focus:border-gray-700 text-white placeholder-gray-500"
            placeholder="Search..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            onKeyDown={handleKeyDown}
          />
          <span className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-500">
            <IoIosSearch
              className="focus:bg-gray-700"
              data-testid="search-icon"
            />
          </span>
        </div>
      </div>
    </div>
  );
}

export default TopMenu;
