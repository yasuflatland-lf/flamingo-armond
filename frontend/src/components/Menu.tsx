import React from "react";
import { Link } from "react-router-dom";
import { IoIosHeart, IoIosHome, IoIosList } from "react-icons/io";
import { AiFillDislike } from "react-icons/ai";
import { FaPlus } from "react-icons/fa";
import { GrValidate } from "react-icons/gr";

interface MenuProps {
  likeMenu: boolean;
}

function Menu({ likeMenu = true }: MenuProps) {
  return (
    <div className="fixed bottom-0 w-full flex flex-col justify-around z-50">
      {likeMenu && (
        <div className="flex justify-around w-full bg-white pt-6 mb-8">
          <button className="bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <IoIosHeart className="text-pink-700 text-4xl" />
          </button>
          <button className="bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <GrValidate className="text-green-700 text-4xl" />
          </button>
          <button className="bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
            <AiFillDislike className="text-green-700 text-4xl" />
          </button>
        </div>
      )}
      <div className="bg-pink-700 flex justify-around pt-2 pb-6">
        <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
          <Link to="/">
            <IoIosHome className="text-2xl" />
          </Link>
        </button>
        <button className="bg-white text-pink-700 rounded-full w-8 h-8 flex items-center justify-center shadow-lg hover:bg-gray-200">
          <Link to="/ldtest">
            <FaPlus className="text-2xl" />
          </Link>
        </button>
        <button className="text-white bg-transparent border-none text-base cursor-pointer hover:text-gray-400">
          <Link to="/list">
            <IoIosList className="text-2xl" />
          </Link>
        </button>
      </div>
    </div>
  );
}

export default Menu;
