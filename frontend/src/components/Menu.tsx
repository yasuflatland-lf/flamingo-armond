import React from "react";
import { Link } from "react-router-dom";
import { IoIosHome } from "react-icons/io";
import { AiFillDislike } from "react-icons/ai";
import { FcLike } from "react-icons/fc";
import { FaPlus } from "react-icons/fa";
import { VscAccount } from "react-icons/vsc";

function Menu() {
  return (
    <>
      <div className="fixed bottom-0 w-full bg-pink-700 flex flex-col justify-around z-50">
        <div className="flex justify-around w-full bg-white py-6">
          <button className="bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <span className="icon-[mdi-light--home] text-4xl">
              <FcLike />
            </span>
          </button>
          <button className="bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <span className="icon-[mdi-light--home] text-4xl">
              <AiFillDislike />
            </span>
          </button>
        </div>

        <div className="flex justify-around py-2">
          <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <Link to="/">
              <span className="icon-[mdi-light--home] text-2xl">
                <IoIosHome />
              </span>
            </Link>
          </button>
          <button className="z-100 bg-white text-pink-700 rounded-full w-16 h-16 flex items-center justify-center shadow-lg hover:bg-gray-200">
            <Link to="/center">
              <FaPlus className="text-3xl" />
            </Link>
          </button>
          <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <Link to="/account">
              <VscAccount className="text-2xl" />
            </Link>
          </button>
        </div>
      </div>
    </>
  );
}

export default Menu;
