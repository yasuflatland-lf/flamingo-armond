import React, { useState } from "react";
import SwipeableCard from "../components/SwipeableCard.tsx";
import TopMenu from "../components/TopMenu.tsx";
import Menu from "../components/Menu.tsx";

function Home() {
  const [cards] = useState([
    { id: 1, content: "Brown Fox Leap Over The Box" },
    { id: 2, content: "Card 2" },
    { id: 3, content: "Card 3" },
  ]);

  const [currentCardIndex, setCurrentCardIndex] = useState(0);

  const handleSwiped = (dir: string) => {
    if (dir === "left") {
      setCurrentCardIndex((prev) => (prev > 0 ? prev - 1 : cards.length - 1));
    } else if (dir === "right") {
      setCurrentCardIndex((prev) => (prev + 1) % cards.length);
    }
  };

  return (
    <>
      <TopMenu />
      {cards.length > 0 && (
        <SwipeableCard
          key={cards[currentCardIndex].id}
          content={cards[currentCardIndex].content}
          onSwiped={handleSwiped}
        />
      )}
      <Menu />
    </>
  );
}

export default Home;
