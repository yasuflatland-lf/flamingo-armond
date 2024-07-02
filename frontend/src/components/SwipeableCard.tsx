import React, { useState } from "react";
import { useSwipeable } from "react-swipeable";
import { IoIosHeart } from "react-icons/io";
import { AiFillDislike } from "react-icons/ai";
import { GrValidate } from "react-icons/gr";
import "./SwipeableCard.css";

interface SwipeableCardProps {
  content: string;
  onSwiped: (dir: string) => void;
}

function SwipeableCard({ content, onSwiped }: SwipeableCardProps) {
  const [isFlipped, setIsFlipped] = useState(false);
  const [watermark, setWatermark] = useState<React.ReactNode>(null);
  const [watermarkColor, setWatermarkColor] = useState("");

  const handlers = useSwipeable({
    onSwipedLeft: () => handleSwipe("left"),
    onSwipedRight: () => handleSwipe("right"),
    onSwipedUp: () => handleSwipe("up"),
    onSwipedDown: () => handleSwipe("down"),
    onSwiping: (eventData) => handleSwiping(eventData),
    onTap: (eventData) => handleTapping(eventData),
  });

  const handleSwipe = (dir: string) => {
    if(dir === "up" ) {
      setWatermark(null);
      setIsFlipped(true);
    } else {
      setWatermark(null);
      onSwiped(dir);
    }
  };

  const handleTapping = ({ dir }: { dir: string }) => {
    setIsFlipped(false);
    console.log(dir);
  };

  const handleSwiping = ({ dir }: { dir: string }) => {
    if (dir === "Left") {
      setWatermark(<IoIosHeart />);
      setWatermarkColor("text-pink-700 opacity-30");
    } else if (dir === "Right") {
      setWatermark(<AiFillDislike />);
      setWatermarkColor("text-green-700 opacity-30");
    } else if (dir === "Down") {
      setWatermark(<GrValidate />);
      setWatermarkColor("text-green-700 opacity-30");
    } else {
      setWatermark(null);
      setWatermarkColor("");
    }
  };

  return (
    <div
      {...handlers}
      className={`swipeable-card ${isFlipped ? "flipped" : ""} justify-center p-8`}
    >
      <div
        data-testid="wartermark-id"
        className={`watermark ${watermarkColor}`}
      >
        {watermark}
      </div>
      <h1 className="font-mono text-4xl font-extrabold swipeable-card-content">
        {content}
      </h1>
      <div className="swipeable-card-back">{content} (Back)</div>
    </div>
  );
}

export default SwipeableCard;
