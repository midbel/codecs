<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<xsl:variable name="list" select="(1, 2, 3)"/>
		<sequence>
			<total>
				<xsl:value-of select="sum($list)"/>
			</total>
		</sequence>
	</xsl:template>
</xsl:stylesheet>