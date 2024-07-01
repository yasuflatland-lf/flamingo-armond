import React from "react";
import { IoIosSearch, IoIosSettings } from "react-icons/io";

function TopMenu() {
  return (
    <div className="bg-pink-700">
      <div className="fixed top-0 w-full flex justify-end py-2.5 pr-4 z-50">
        <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
          <span className="icon-[mdi-light--home] text-2xl">
            <IoIosSettings />
          </span>
        </button>
      </div>
      <div className="pt-12 px-4 pb-6">
        <div className="relative">
          <input
            type="text"
            className="w-full pl-10 pr-4 py-2 border border-gray-700 rounded-lg focus:outline-none focus:border-blue-500 text-white placeholder-gray-500"
            placeholder="Search..."
          />
          <span className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-500">
            <IoIosSearch />
          </span>
        </div>
      </div>
    </div>
  );
}

export default TopMenu;
